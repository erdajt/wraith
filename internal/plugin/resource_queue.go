package plugin

import (
	"context"
	"slices"
)

type queuedResourceEvent struct {
	event    ResourceEvent
	lossless bool
}

// Emit either preserves a protocol event through backpressure or coalesces capture notifications.
func (s *resourceSink) Emit(kind string, data map[string]any, lossless bool) error {
	detached, err := cloneResourceValue(data)
	if err != nil {
		return err
	}
	for {
		s.manager.mu.Lock()
		record := s.manager.records[s.ID]
		if record == nil || record.state != "running" || s.manager.closed {
			s.manager.mu.Unlock()
			return context.Canceled
		}
		if s.manager.progress >= 256 && lossless {
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
					s.manager.queue = slices.Delete(s.manager.queue, i, i+1)
					s.manager.queue = append(s.manager.queue, queuedResourceEvent{event: event})
					s.manager.mu.Unlock()
					return nil
				}
			}
			if s.manager.progress >= 256 {
				record.dropped++
				s.manager.mu.Unlock()
				return nil
			}
		}
		s.manager.queue = append(s.manager.queue, queuedResourceEvent{event: event, lossless: lossless})
		s.manager.progress++
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
		m.queue = m.queue[1:]
		if !queued.event.Final {
			m.progress--
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
