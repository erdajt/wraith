package distribution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type gitOutput struct {
	bytes.Buffer
	truncated bool
}

// Write drains process output while retaining at most 64 kib for diagnostics.
func (b *gitOutput) Write(data []byte) (int, error) {
	length := len(data)
	remaining := (64 << 10) - b.Len()
	if len(data) > remaining {
		data, b.truncated = data[:remaining], true
	}
	_, _ = b.Buffer.Write(data)
	return length, nil
}

// runGit isolates seed operations from inherited git configuration and bounds their lifetime.
func runGit(parent context.Context, directory string, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	policy := []string{
		"--git-dir=" + filepath.Join(directory, ".git"), "--work-tree=" + directory,
		"--no-optional-locks", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false",
		"-c", "submodule.recurse=false", "-c", "protocol.allow=never", "-c", "protocol.file.allow=always",
	}
	command := exec.CommandContext(ctx, "git", append(policy, arguments...)...)
	command.Dir = directory
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"), "LANG=C", "LC_ALL=C", "HOME=" + directory,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_COUNT=0", "GIT_TERMINAL_PROMPT=0", "GIT_ATTR_NOSYSTEM=1",
		"GIT_NO_REPLACE_OBJECTS=1",
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = time.Second
	var stdout, stderr gitOutput
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", arguments[0], err, stderr.String())
	}
	if stdout.truncated || stderr.truncated {
		return "", errors.New("git output exceeded 64 kib")
	}
	return stdout.String(), nil
}
