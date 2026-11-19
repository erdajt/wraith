package plugin

import (
	"errors"
	"slices"
)

// enqueue accepts bounded writes in order; closing stdin remains ordered behind earlier writes.
func (p *managedProcess) enqueue(data []byte, closeInput bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running || p.input == nil {
		return errors.New("process stdin is not running or was not opened")
	}
	if p.inputClosing {
		if closeInput {
			return nil
		}
		return errors.New("process stdin is closed")
	}
	if len(data) > 64<<10 || p.inputBytes+len(data) > 1<<20 {
		return errors.New("process stdin exceeds 64 kib per write or 1 mib pending")
	}
	request := processInput{data: slices.Clone(data), close: closeInput}
	select {
	case p.queue <- request:
		p.inputBytes += len(data)
		p.inputClosing = closeInput
		return nil
	default:
		return errors.New("process stdin queue is full")
	}
}

// output returns a detached byte window from one stream, including completed processes.
func (p *managedProcess) output(stream string, offset int64, limit int) (map[string]any, error) {
	p.mu.Lock()
	output := p.stdout
	if stream == "stderr" {
		output = p.stderr
	} else if stream != "stdout" {
		p.mu.Unlock()
		return nil, errors.New("process stream must be stdout or stderr")
	}
	p.mu.Unlock()
	if output == nil {
		return nil, errors.New("process has not started")
	}
	return output.query(offset, limit)
}
