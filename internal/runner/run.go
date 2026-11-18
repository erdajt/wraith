package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const TailLimit = 64 << 10

type capture struct {
	limit, tailLimit   int
	sink               io.Writer
	mu                 sync.Mutex
	tail               string
	observed, tailLine int64
	text               strings.Builder
	truncated          bool
	emit               func(string)
}

// Write retains a bounded prefix and tail while forwarding output to the stream callback.
func (c *capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	limit, tailLimit := c.limit, c.tailLimit
	if limit == 0 {
		limit = OutputLimit
	}
	if tailLimit == 0 {
		tailLimit = TailLimit
	}

	n := min(len(p), limit-c.text.Len())
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
	if len(c.tail) > tailLimit {
		removed := len(c.tail) - tailLimit
		c.tailLine += int64(strings.Count(c.tail[:removed], "\n"))
		c.tail = c.tail[removed:]
	}

	if c.emit != nil {
		c.emit(string(p))
	}
	return len(p), nil
}

// prepare validates a command and persists its starting state before execution.
func (h History) prepare(r Record) (Record, *liveRun, error) {
	if strings.TrimSpace(r.Command) == "" || strings.ContainsRune(r.Command, 0) {
		return r, nil, errors.New("command is empty or contains a null byte")
	}
	if !validID.MatchString(r.ID) {
		return r, nil, errors.New("invalid command id")
	}

	if _, err := os.Stat(filepath.Join(h.Root, r.ID+".json.gz")); !errors.Is(err, os.ErrNotExist) {
		if err != nil {
			return r, nil, err
		}
		return r, nil, errors.New("command id already exists in history")
	}

	if r.CaptureLimit == 0 {
		r.CaptureLimit = OutputLimit
	}
	if r.TailLimit == 0 {
		r.TailLimit = TailLimit
	}
	if err := r.validateLimits(); err != nil {
		return r, nil, err
	}

	r.Started, r.Status, r.ExitCode = time.Now().UTC(), "starting", -1
	live, err := h.openLive(r)
	if err == nil {
		err = h.Save(r)
	}
	if err != nil {
		r.Finished, r.Status, r.Error = time.Now().UTC(), "failed", err.Error()
		if live != nil {
			_ = live.checkpoint(r, &capture{})
			_ = h.Save(r)
			live.close()
		}
		return r, nil, err
	}
	return r, live, nil
}

// Run executes a command synchronously and persists its captured output and status.
func (h History) Run(ctx context.Context, r Record, emit func(string)) (Record, error) {
	r, live, err := h.prepare(r)
	if err != nil {
		return r, err
	}
	return h.execute(ctx, r, live, emit, nil)
}

// execute runs a process group with output checkpoints and records its final status.
func (h History) execute(ctx context.Context, r Record, live *liveRun, emit func(string), started func()) (Record, error) {
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
	output := &capture{emit: emit, sink: live.prefix, limit: r.CaptureLimit, tailLimit: r.TailLimit}
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
	err := cmd.Start()
	if err == nil {
		if started != nil {
			started()
		}
		err = cmd.Wait()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

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

// Write captures output and checkpoints the journal after each write threshold.
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

// validateLimits checks capture limits and retained output sizes.
func (r Record) validateLimits() error {
	captureLimit, tailLimit := r.CaptureLimit, r.TailLimit
	if captureLimit == 0 {
		captureLimit = OutputLimit
	}
	if tailLimit == 0 {
		tailLimit = TailLimit
	}
	if captureLimit < 64<<10 || captureLimit > MaxOutputLimit || tailLimit < 4<<10 || tailLimit > 1<<20 || tailLimit > captureLimit {
		return errors.New("invalid command capture limits")
	}
	if len(r.Output) > captureLimit || r.Captured < 0 || r.Captured > captureLimit || len(r.Tail) > tailLimit {
		return errors.New("command output exceeds capture limits")
	}
	return nil
}
