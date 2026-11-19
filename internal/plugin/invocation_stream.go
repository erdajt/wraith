package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
)

// finish retains one terminal result and cancels every managed child before waking consumers.
func (m *invocationManager) finish(id string, data json.RawMessage, err error) error {
	m.mu.Lock()
	record := m.records[id]
	if record == nil || record.terminal != nil {
		m.mu.Unlock()
		return errors.New("invocation is already completed or released")
	}
	record.sequence++
	event := InvocationEvent{Sequence: record.sequence, Data: bytes.Clone(data), Final: true}
	if err != nil {
		event.Error = err.Error()
	}
	record.terminal = &event
	children := make([]string, 0, len(record.children))
	for child := range record.children {
		children = append(children, child)
	}
	record.children = nil
	record.stop()
	record.cancel()
	select {
	case record.wake <- struct{}{}:
	default:
	}
	m.mu.Unlock()
	m.cancelChildren(children)
	return nil
}

// commit publishes ordered callback replies without blocking the VM on a slow consumer.
func (m *invocationManager) commit(effect InvocationEffect) error {
	data, err := invocationPayload(effect.Data)
	if err != nil {
		_ = m.finish(effect.ID, nil, err)
		return err
	}
	if effect.Kind == "finish" {
		return m.finish(effect.ID, data, nil)
	}
	if effect.Kind == "fail" {
		if effect.Error == "" || len(effect.Error) > 4096 {
			return errors.New("invocation failure requires a message within 4096 bytes")
		}
		return m.finish(effect.ID, nil, errors.New(effect.Error))
	}
	if effect.Kind != "emit" {
		return errors.New("unknown invocation reply kind")
	}
	m.mu.Lock()
	record := m.records[effect.ID]
	if record == nil || record.terminal != nil || record.context.Err() != nil {
		m.mu.Unlock()
		return errors.New("invocation is no longer active")
	}
	if len(record.queue) >= 64 || record.bytes+len(data) > 1<<20 {
		m.mu.Unlock()
		err := errors.New("invocation stream exceeded 64 chunks or 1 mib of queued output")
		_ = m.finish(effect.ID, nil, err)
		return err
	}
	record.sequence++
	record.queue = append(record.queue, InvocationEvent{Sequence: record.sequence, Data: data})
	record.bytes += len(data)
	select {
	case record.wake <- struct{}{}:
	default:
	}
	m.mu.Unlock()
	return nil
}

// Next reads committed deltas before the terminal result and releases each queued byte charge.
func (i *Invocation) Next(ctx context.Context) (InvocationEvent, error) {
	for {
		if err := ctx.Err(); err != nil {
			return InvocationEvent{}, err
		}
		i.manager.mu.Lock()
		record := i.manager.records[i.ID]
		if record == nil {
			i.manager.mu.Unlock()
			return InvocationEvent{}, io.EOF
		}
		if len(record.queue) > 0 {
			event := record.queue[0]
			record.queue[0] = InvocationEvent{}
			record.queue = record.queue[1:]
			record.bytes -= len(event.Data)
			i.manager.mu.Unlock()
			return event, nil
		}
		if record.terminal != nil {
			if record.consumed {
				i.manager.mu.Unlock()
				return InvocationEvent{}, io.EOF
			}
			record.consumed = true
			event := *record.terminal
			event.Data = bytes.Clone(event.Data)
			i.manager.mu.Unlock()
			return event, nil
		}
		wake := record.wake
		i.manager.mu.Unlock()
		select {
		case <-ctx.Done():
			return InvocationEvent{}, ctx.Err()
		case <-wake:
		}
	}
}
