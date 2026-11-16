package runner

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const TailLimit = 64 << 10

type capture struct {
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
	cmd := exec.CommandContext(ctx, r.Shell, "-c", r.Command)
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
	output := &capture{emit: emit}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
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
	if saveErr := h.Save(r); saveErr != nil {
		return r, saveErr
	}
	return r, nil
}
