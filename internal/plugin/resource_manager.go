package plugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

type ResourceEvent struct {
	ID       string
	Revision uint64
	Sequence uint64
	Kind     string
	Data     map[string]any
	Final    bool
}

type resourceResult struct {
	Data map[string]any
	Err  error
}

type resourceWork func(context.Context, *resourceSink) resourceResult

type managedResource struct {
	id, owner, kind, state string
	ctx                    context.Context
	cancel                 context.CancelFunc
	work                   resourceWork
	sequence               uint64
	activated, done        bool
	result                 resourceResult
	dropped                uint64
	control                any
}

type resourceManager struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	records  map[string]*managedResource
	prefix   string
	serial   uint64
	closed   bool
	events   chan ResourceEvent
	wake     chan struct{}
	changed  chan struct{}
	queue    []queuedResourceEvent
	progress int
}

type resourceSink struct {
	ID, Owner string
	manager   *resourceManager
	ctx       context.Context
}

// newResourceManager owns managed work independently of the Lua actor's responsiveness.
func newResourceManager(parent context.Context) *resourceManager {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(parent)
	manager := &resourceManager{
		ctx: ctx, cancel: cancel, records: map[string]*managedResource{}, prefix: hex.EncodeToString(nonce[:]),
		events: make(chan ResourceEvent), wake: make(chan struct{}, 1), changed: make(chan struct{}),
	}
	go manager.deliver()
	return manager
}

// reserve records owned work without starting external operations.
func (m *resourceManager) reserve(owner, kind string, work resourceWork) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.ctx.Err() != nil {
		return "", context.Canceled
	}
	active := 0
	for _, record := range m.records {
		if record.state == "reserved" || record.state == "running" {
			active++
		}
	}
	if len(m.records) >= 128 || active >= 32 {
		return "", errors.New("resource limit reached; release completed handles")
	}
	m.serial++
	id := fmt.Sprintf("resource.%s.%d", m.prefix, m.serial)
	ctx, cancel := context.WithCancel(m.ctx)
	m.records[id] = &managedResource{id: id, owner: owner, kind: kind, state: "reserved", ctx: ctx, cancel: cancel, work: work}
	return id, nil
}

// activate starts a reserved operation once after the host accepts its action chain.
func (m *resourceManager) activate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := m.records[id]
	if m.closed || m.ctx.Err() != nil {
		return context.Canceled
	}
	if record == nil {
		return errors.New("resource no longer exists")
	}
	if record.activated {
		return nil
	}
	if record.state != "reserved" {
		return fmt.Errorf("resource cannot start in state %s", record.state)
	}
	record.activated, record.state = true, "running"
	go m.run(record)
	return nil
}

// run converts worker failures into one retained terminal result.
func (m *resourceManager) run(record *managedResource) {
	result := resourceResult{}
	defer func() {
		if failure := recover(); failure != nil {
			result.Err = fmt.Errorf("resource worker panic: %v", failure)
		}
		m.finish(record, result)
	}()
	result = record.work(record.ctx, &resourceSink{ID: record.id, Owner: record.owner, manager: m, ctx: record.ctx})
}

// finish retains completion even when cancellation already delivered the terminal event.
func (m *resourceManager) finish(record *managedResource, result resourceResult) {
	detached, err := cloneResourceValue(result.Data)
	if err != nil {
		result = resourceResult{Err: err}
	} else {
		result.Data = detached.(map[string]any)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	record.done = true
	if m.records[record.id] != record || m.closed {
		return
	}
	if record.state == "cancelled" {
		return
	}
	record.state = "completed"
	if result.Err != nil {
		record.state = "failed"
	}
	record.result = result
	m.terminal(record)
}

// Cancel requests termination without making completion mean that the worker has already exited.
func (m *resourceManager) Cancel(id, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := m.records[id]
	if record == nil || record.owner != owner {
		return errors.New("resource is missing or belongs to another owner")
	}
	if record.state == "completed" || record.state == "failed" || record.state == "cancelled" {
		return nil
	}
	record.cancel()
	record.state = "cancelled"
	record.result = resourceResult{Err: context.Canceled}
	if record.activated {
		m.terminal(record)
	} else {
		record.done = true
	}
	return nil
}

// Status returns detached lifecycle information while keeping completed handles queryable.
func (m *resourceManager) Status(id, owner string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record := m.records[id]
	if record == nil || record.owner != owner {
		return nil, errors.New("resource is missing or belongs to another owner")
	}
	result := map[string]any{"id": id, "kind": record.kind, "state": record.state, "revision": uint64(1), "sequence": record.sequence, "done": record.done, "dropped": record.dropped}
	if record.result.Err != nil {
		result["error"] = record.result.Err.Error()
	}
	return result, nil
}

// discard invalidates handles and unblocks their workers without waiting for the VM.
func (m *resourceManager) discard(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		if record := m.records[id]; record != nil {
			record.cancel()
			delete(m.records, id)
		}
	}
	m.removeQueued(ids)
}

// close cancels managed work without waiting for native Lua callbacks or process reaping.
func (m *resourceManager) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	m.cancel()
	for _, record := range m.records {
		record.cancel()
	}
	m.records = map[string]*managedResource{}
	m.queue = nil
	m.signalChanged()
}
