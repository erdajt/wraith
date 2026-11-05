package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Event struct {
	Data []byte
	Err  error
}
type PTY struct {
	file   *os.File
	cmd    *exec.Cmd
	Events chan Event
	done   chan struct{}
	waited chan struct{}
	once   sync.Once
}

func Start(argv []string, dir string, w, h int) (*PTY, error) {
	if len(argv) == 0 {
		return nil, errors.New("empty shell command")
	}
	f, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	if err = unix.IoctlSetPointerInt(int(f.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		f.Close()
		return nil, err
	}
	n, err := unix.IoctlGetInt(int(f.Fd()), unix.TIOCGPTN)
	if err != nil {
		f.Close()
		return nil, err
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		f.Close()
		return nil, err
	}
	defer slave.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "WRAITH_TERMINAL=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	p := &PTY{file: f, cmd: cmd, Events: make(chan Event, 128), done: make(chan struct{}), waited: make(chan struct{})}
	if err = p.Resize(w, h); err != nil {
		f.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		f.Close()
		return nil, err
	}
	go func() { _ = cmd.Wait(); close(p.waited); _ = f.Close() }()
	go p.read()
	return p, nil
}

func (p *PTY) read() {
	defer close(p.Events)
	buf := make([]byte, 8192)
	for {
		n, err := p.file.Read(buf)
		if n > 0 {
			select {
			case p.Events <- Event{Data: append([]byte(nil), buf[:n]...)}:
			case <-p.done:
				return
			}
		}
		if err != nil {
			if errors.Is(err, unix.EIO) || errors.Is(err, os.ErrClosed) {
				err = io.EOF
			}
			select {
			case p.Events <- Event{Err: err}:
			case <-p.done:
			}
			return
		}
	}
}
func (p *PTY) Write(data []byte) error { _, err := p.file.Write(data); return err }
func (p *PTY) Resize(w, h int) error {
	return unix.IoctlSetWinsize(int(p.file.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(max(1, min(w, 65535))), Row: uint16(max(1, min(h, 65535)))})
}
func (p *PTY) Close() {
	p.once.Do(func() {
		close(p.done)
		_ = p.file.Close()
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGHUP)
		go func() {
			select {
			case <-p.waited:
			case <-time.After(time.Second):
				_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
			}
		}()
	})
}
