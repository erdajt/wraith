package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"
)

// newInvocationManager separates host waits from the Lua actor and managed worker budget.
func newInvocationManager(prefix string, cancelChildren func([]string)) *invocationManager {
	return &invocationManager{prefix: prefix, records: map[string]*invocationRecord{}, starts: make(chan InvocationStart, 128), cancelChildren: cancelChildren}
}

// begin reserves a bounded handle before making its callback visible to the host.
func (m *invocationManager) begin(parent context.Context, request InvocationRequest) (*Invocation, error) {
	data, err := invocationPayload(request.Data)
	if err != nil {
		return nil, err
	}
	timeout := request.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
		if request.Binding.Method == "models" || request.Binding.Method == "show" {
			timeout = 30 * time.Second
		}
	}
	if timeout < time.Millisecond || timeout > 24*time.Hour {
		return nil, errors.New("invocation timeout must be between 1 ms and 24 hours")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	m.mu.Lock()
	active := 0
	for _, record := range m.records {
		if record.terminal == nil && record.context.Err() == nil {
			active++
		}
	}
	if m.closed || ctx.Err() != nil || active >= 32 || len(m.records) >= 128 {
		m.mu.Unlock()
		cancel()
		return nil, errors.New("invocation runtime is closed, canceled or at capacity")
	}
	m.serial++
	id := fmt.Sprintf("%s.invocation.%d", m.prefix, m.serial)
	record := &invocationRecord{context: ctx, binding: request.Binding, data: bytes.Clone(data), cancel: cancel, wake: make(chan struct{}, 1), children: map[string]bool{}}
	m.records[id] = record
	select {
	case m.starts <- InvocationStart{ID: id, Binding: request.Binding, Data: data}:
	default:
		delete(m.records, id)
		m.mu.Unlock()
		cancel()
		return nil, ErrQueueFull
	}
	record.stop = context.AfterFunc(ctx, func() { m.finish(id, nil, ctx.Err()) })
	m.mu.Unlock()
	return &Invocation{ID: id, manager: m}, nil
}

// valid accepts live operations only; terminal handles remain readable but cannot emit effects.
func (m *invocationManager) valid(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := m.records[id]
	return !m.closed && record != nil && record.terminal == nil && record.context.Err() == nil
}

// attach includes reservations in cancellation even before host activation.
func (m *invocationManager) attach(id, child string) error {
	m.mu.Lock()
	record := m.records[id]
	if record == nil || record.terminal != nil || record.context.Err() != nil || m.closed {
		m.mu.Unlock()
		m.cancelChildren([]string{child})
		return errors.New("invocation is no longer active")
	}
	record.children[child] = true
	m.mu.Unlock()
	return nil
}

// discard removes retained data and cancels children without waiting for Lua cleanup.
func (m *invocationManager) discard(id string) {
	m.mu.Lock()
	record := m.records[id]
	if record == nil {
		m.mu.Unlock()
		return
	}
	delete(m.records, id)
	m.removeStart(id)
	children := slices.Collect(maps.Keys(record.children))
	record.queue = nil
	record.bytes = 0
	record.cancel()
	record.stop()
	select {
	case record.wake <- struct{}{}:
	default:
	}
	m.mu.Unlock()
	m.cancelChildren(children)
}

// Close ends consumption and releases the invocation, including a queued unstarted callback.
func (i *Invocation) Close() { i.manager.discard(i.ID) }

// close invalidates every handle independently of the VM owner.
func (m *invocationManager) close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	close(m.starts)
	ids := make([]string, 0, len(m.records))
	for id := range m.records {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.discard(id)
	}
}

// removeStart releases queued notices while retaining the order of other live starts.
func (m *invocationManager) removeStart(id string) {
	var retained []InvocationStart
	for {
		select {
		case start, ok := <-m.starts:
			if !ok {
				return
			}
			if start.ID != id && !m.closed {
				retained = append(retained, start)
			}
		default:
			for _, start := range retained {
				m.starts <- start
			}
			return
		}
	}
}
