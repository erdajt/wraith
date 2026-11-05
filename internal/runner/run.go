package runner

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type capture struct {
	text      strings.Builder
	truncated bool
	emit      func(string)
}

func (c *capture) Write(p []byte) (int, error) {
	n := min(len(p), OutputLimit-c.text.Len())
	if n < len(p) {
		c.truncated = true
	}
	if n > 0 {
		c.text.Write(p[:n])
		if c.emit != nil {
			c.emit(string(p[:n]))
		}
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	}
	if saveErr := h.Save(r); saveErr != nil {
		return r, saveErr
	}
	return r, nil
}
