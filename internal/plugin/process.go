package plugin

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type processOptions struct {
	argv        []string
	directory   string
	environment []string
	timeout     time.Duration
	limit       int
	protocol    bool
	stdin       bool
}

type processInput struct {
	data  []byte
	close bool
}

type managedProcess struct {
	mu             sync.Mutex
	options        processOptions
	stdout, stderr *processOutput
	input          io.WriteCloser
	queue          chan processInput
	stop           chan struct{}
	inputDone      chan struct{}
	inputBytes     int
	inputClosing   bool
	running        bool
	inputErr       error
}

// newManagedProcess allocates bounded input state without starting a command.
func newManagedProcess(options processOptions) *managedProcess {
	return &managedProcess{
		options: options, queue: make(chan processInput, 32),
		stop: make(chan struct{}), inputDone: make(chan struct{}),
	}
}

// run owns one process group and keeps its stdio separate from the terminal UI.
func (p *managedProcess) run(parent context.Context, sink *resourceSink) resourceResult {
	ctx, cancel := context.WithCancel(parent)
	if p.options.timeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(parent, p.options.timeout)
	}
	defer cancel()
	streamSink := *sink
	streamSink.ctx = ctx
	ready := make(chan struct{})
	stdout := &processOutput{limit: p.options.limit, stream: "stdout", protocol: p.options.protocol, sink: &streamSink, ready: ready}
	stderr := &processOutput{limit: p.options.limit, stream: "stderr", protocol: p.options.protocol, sink: &streamSink, ready: ready}
	p.mu.Lock()
	p.stdout, p.stderr = stdout, stderr
	p.mu.Unlock()

	cmd := exec.CommandContext(ctx, p.options.argv[0], p.options.argv[1:]...)
	cmd.Dir, cmd.Env = p.options.directory, p.options.environment
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	cmd.WaitDelay = time.Second
	var input io.WriteCloser
	var err error
	if p.options.stdin {
		input, err = cmd.StdinPipe()
		if err != nil {
			return resourceResult{Err: err}
		}
	}
	if err := cmd.Start(); err != nil {
		if input != nil {
			_ = input.Close()
		}
		return resourceResult{Err: err}
	}
	p.mu.Lock()
	p.running, p.input = true, input
	p.mu.Unlock()
	if input != nil {
		go p.writeInput(ctx, &streamSink, input)
	} else {
		close(p.inputDone)
	}
	if err := streamSink.Emit("started", map[string]any{"pid": cmd.Process.Pid}, true); err != nil {
		cancel()
	}

	close(ready)
	err = cmd.Wait()
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
	close(p.stop)
	if input != nil {
		_ = input.Close()
	}
	<-p.inputDone
	p.mu.Lock()
	inputErr := p.inputErr
	for len(p.queue) > 0 {
		<-p.queue
	}
	p.inputBytes = 0
	p.mu.Unlock()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	data := map[string]any{"pid": cmd.Process.Pid, "exit_code": cmd.ProcessState.ExitCode()}
	if inputErr != nil {
		data["stdin_error"] = inputErr.Error()
	}
	return resourceResult{Data: data, Err: err}
}

// writeInput serializes accepted writes and reports pipe failures without entering the Lua VM.
func (p *managedProcess) writeInput(ctx context.Context, sink *resourceSink, input io.WriteCloser) {
	defer close(p.inputDone)
	defer input.Close()
	for {
		select {
		case <-p.stop:
			return
		case <-ctx.Done():
			return
		case request := <-p.queue:
			if request.close {
				return
			}
			written := 0
			var err error
			for written < len(request.data) {
				var count int
				count, err = input.Write(request.data[written:])
				written += count
				if err != nil || count == 0 {
					if err == nil {
						err = io.ErrShortWrite
					}
					break
				}
			}
			p.mu.Lock()
			p.inputBytes -= len(request.data)
			if err != nil {
				p.inputErr, p.inputClosing = err, true
			}
			p.mu.Unlock()
			if err != nil {
				_ = sink.Emit("stdin", map[string]any{"written": written, "error": err.Error()}, false)
				return
			}
		}
	}
}
