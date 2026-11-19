package plugin

import (
	"context"
	"errors"
	"slices"
)

type queuedResourceEvent struct {
	event    ResourceEvent
	lossless bool
	bytes    int
}

// Emit either preserves a protocol event through backpressure or coalesces capture notifications.
func (s *resourceSink) Emit(kind string, data map[string]any, lossless bool) error {
	detached, err := cloneResourceValue(data)
	if err != nil {
		return err
	}
	size := resourceValueSize(detached) + len(kind) + len(s.ID)
	if size > 8<<20 {
		return errors.New("resource progress event exceeds 8 mib")
	}
	for {
		s.manager.mu.Lock()
		record := s.manager.records[s.ID]
		if record == nil || record.state != "running" || s.manager.closed {
			s.manager.mu.Unlock()
			return context.Canceled
		}
		if (s.manager.progress >= 256 || s.manager.progressBytes+size > 8<<20) && lossless {
			changed := s.manager.changed
			s.manager.mu.Unlock()
			select {
			case <-s.ctx.Done():
				return s.ctx.Err()
			case <-changed:
				continue
			}
		}
		record.sequence++
		event := ResourceEvent{ID: s.ID, Revision: 1, Sequence: record.sequence, Kind: kind, Data: detached.(map[string]any)}
		if !lossless {
			for i, queued := range s.manager.queue {
				if !queued.lossless && !queued.event.Final && queued.event.ID == s.ID && queued.event.Kind == kind {
					if s.manager.progressBytes-queued.bytes+size > 8<<20 {
						record.dropped++
						s.manager.mu.Unlock()
						return nil
					}
					s.manager.progressBytes += size - queued.bytes
					s.manager.queue = slices.Delete(s.manager.queue, i, i+1)
					s.manager.queue = append(s.manager.queue, queuedResourceEvent{event: event, bytes: size})
					s.manager.signalChanged()
					s.manager.mu.Unlock()
					return nil
				}
			}
			if s.manager.progress >= 256 || s.manager.progressBytes+size > 8<<20 {
				record.dropped++
				s.manager.mu.Unlock()
				return nil
			}
		}
		s.manager.queue = append(s.manager.queue, queuedResourceEvent{event: event, lossless: lossless, bytes: size})
		s.manager.progress++
		s.manager.progressBytes += size
		s.manager.signalDelivery()
		s.manager.mu.Unlock()
		return nil
	}
}

// terminal uses reserved control capacity rather than waiting behind a full progress queue.
func (m *resourceManager) terminal(record *managedResource) {
	record.sequence++
	result, err := cloneResourceValue(record.result.Data)
	if err != nil {
		record.result.Err = err
		result = nil
	}
	data := map[string]any{"state": record.state, "done": record.done, "result": result}
	if record.callbackError != "" {
		data["callback_error"] = record.callbackError
	}
	if record.result.Err != nil {
		data["error"] = record.result.Err.Error()
	}
	m.queue = append(m.queue, queuedResourceEvent{event: ResourceEvent{ID: record.id, Revision: 1, Sequence: record.sequence, Kind: record.kind, Data: data, Final: true}})
	m.signalDelivery()
}

// deliver keeps event order while allowing cancellation to stop a stalled host consumer.
func (m *resourceManager) deliver() {
	defer close(m.events)
	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return
		}
		if len(m.queue) == 0 {
			m.mu.Unlock()
			select {
			case <-m.ctx.Done():
				return
			case <-m.wake:
				continue
			}
		}
		queued := m.queue[0]
		m.queue[0] = queuedResourceEvent{}
		m.queue = m.queue[1:]
		if !queued.event.Final {
			m.progress--
			m.progressBytes -= queued.bytes
		}
		m.signalChanged()
		m.mu.Unlock()
		select {
		case <-m.ctx.Done():
			return
		case m.events <- queued.event:
		}
	}
}

// signalDelivery wakes the single event dispatcher without blocking a producer.
func (m *resourceManager) signalDelivery() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// signalChanged releases protocol producers waiting for queue capacity.
func (m *resourceManager) signalChanged() {
	close(m.changed)
	m.changed = make(chan struct{})
}

// removeQueued removes invalidated events and releases blocked protocol producers.
func (m *resourceManager) removeQueued(ids []string) {
	m.queue = slices.DeleteFunc(m.queue, func(queued queuedResourceEvent) bool {
		removed := slices.Contains(ids, queued.event.ID)
		if removed && !queued.event.Final {
			m.progress--
			m.progressBytes -= queued.bytes
		}
		return removed
	})
	m.signalChanged()
}

// valid keeps completed events usable until their handle is released or its owner unloads.
func (m *resourceManager) valid(event ResourceEvent) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := m.records[event.ID]
	return !m.closed && record != nil && event.Revision == 1 && event.Sequence > 0 && event.Sequence <= record.sequence
}

// resourceValueSize charges detached payload bytes and fixed overhead for each retained value.
func resourceValueSize(value any) int {
	size := 32
	switch value := value.(type) {
	case string:
		size += len(value)
	case []byte:
		size += len(value)
	case []any:
		for _, item := range value {
			size += resourceValueSize(item)
		}
	case map[string]any:
		for key, item := range value {
			size += len(key) + resourceValueSize(item)
		}
	}
	return size
}
