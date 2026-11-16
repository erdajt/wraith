package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const TailLimit = 64 << 10

type capture struct {
	sink               io.Writer
	mu                 sync.Mutex
	tail               string
	observed, tailLine int64
	text               strings.Builder
	truncated          bool
	emit               func(string)
}

func (c *capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := min(len(p), OutputLimit-c.text.Len())
	if n < len(p) {
		c.truncated = true
	}
	if n > 0 {
		if c.sink != nil {
			if _, err := c.sink.Write(p[:n]); err != nil {
				return 0, err
			}
		}
		c.text.Write(p[:n])
	}
	c.observed += int64(len(p))
	c.tail += string(p)
	if len(c.tail) > TailLimit {
		removed := len(c.tail) - TailLimit
		c.tailLine += int64(strings.Count(c.tail[:removed], "\n"))
		c.tail = c.tail[removed:]
	}
	if c.emit != nil {
		c.emit(string(p))
	}
	return len(p), nil
}

func (h History) Run(ctx context.Context, r Record, emit func(string)) (Record, error) {
	if strings.TrimSpace(r.Command) == "" || strings.ContainsRune(r.Command, 0) {
		return r, errors.New("command is empty or contains a null byte")
	}
	r.Started = time.Now().UTC()
	r.Status = "running"
	r.ExitCode = -1
	if err := h.Save(r); err != nil {
		return r, err
	}
	live, err := h.openLive(r)
	if err != nil {
		return r, err
	}
	defer live.close()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(child, r.Shell, "-c", r.Command)
	cmd.Dir = r.Directory
	cmd.Env = r.Environment
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	cmd.WaitDelay = time.Second
	output := &capture{emit: emit, sink: live.prefix}
	writer := &journalWriter{output: output, live: live, record: r}
	cmd.Stdout, cmd.Stderr = writer, writer
	stop := make(chan struct{})
	checkpointed := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				checkpointed <- nil
				return
			case <-ticker.C:
				if err := live.checkpoint(r, output); err != nil {
					cancel()
					checkpointed <- err
					return
				}
			}
		}
	}()
	err = cmd.Run()
	close(stop)
	checkpointErr := <-checkpointed
	if checkpointErr != nil {
		err = fmt.Errorf("checkpoint output: %w", checkpointErr)
	}
	r.Finished = time.Now().UTC()
	r.Output = output.text.String()
	r.Captured = len(r.Output)
	r.Tail, r.Observed = output.tail, output.observed
	r.TailOffset, r.TailLine = output.observed-int64(len(output.tail)), output.tailLine+1
	r.Truncated = output.truncated
	r.Status = "completed"
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		r.Error = err.Error()
		r.Status = "failed"
	}
	if ctx.Err() != nil {
		r.Status = "cancelled"
		r.Error = ctx.Err().Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.Status = "timed_out"
		}
	}
	if saveErr := live.checkpoint(r, output); saveErr != nil {
		return r, saveErr
	}
	if saveErr := h.Save(r); saveErr != nil {
		return r, saveErr
	}
	return r, os.RemoveAll(live.directory)
}

type journalWriter struct {
	output  *capture
	live    *liveRun
	record  Record
	written int
}

func (w *journalWriter) Write(p []byte) (int, error) {
	n, err := w.output.Write(p)
	if err != nil {
		return n, err
	}
	w.written += n
	if w.written >= 256<<10 {
		w.written = 0
		if err = w.live.checkpoint(w.record, w.output); err != nil {
			return n, err
		}
	}
	return n, nil
}
