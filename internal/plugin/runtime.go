package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

type Command struct {
	Name, Description, Callback string
}

type Manifest struct {
	Config     config.Config
	Commands   []Command
	Callbacks  []string
	Overridden []string
}

type Effect struct {
	Name, Argument string
	Config         *config.Config
}

type Result struct {
	Effects []Effect
	Err     error
}

type request struct {
	callback, event string
	data            []byte
	config          config.Config
	result          chan Result
}

type Runtime struct {
	queue    chan request
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	pending  map[string]bool
	manifest Manifest
}

type engine struct {
	L         *lua.LState
	manifest  Manifest
	functions map[string]*lua.LFunction
	events    map[string][]string
	cleanup   []*lua.LFunction
	actions   map[string]bool
	effects   []Effect
	snapshot  any
	setup     bool
	closing   bool
	serial    uint64
}

// Open loads one candidate on its VM owner; callers install it only after validating its manifest.
func Open(parent context.Context, directory string, c config.Config, actions []string) (*Runtime, error) {
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var detached config.Config
	if err := json.Unmarshal(encoded, &detached); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	r := &Runtime{queue: make(chan request, 128), done: make(chan struct{}), ctx: ctx, cancel: cancel, pending: map[string]bool{}}
	ready := make(chan error, 1)
	go r.serve(directory, detached, actions, ready)
	select {
	case err := <-ready:
		if err != nil {
			r.Close()
			return nil, err
		}
		return r, nil
	case <-ctx.Done():
		r.Close()
		return nil, ctx.Err()
	}
}

// Manifest returns an independent copy of the candidate's registrations and effective settings.
func (r *Runtime) Manifest() Manifest {
	data, _ := json.Marshal(r.manifest)
	var out Manifest
	_ = json.Unmarshal(data, &out)
	return out
}

// Submit queues a callback or coalesced event without blocking the ui or retaining mutable state.
func (r *Runtime) Submit(callback, event string, snapshot any, c config.Config) (<-chan Result, error) {
	data, err := json.Marshal(snapshot)
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("plugin snapshot exceeds value limits")
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var copy config.Config
	if err := json.Unmarshal(encoded, &copy); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if event != "" && r.pending[event] {
		return nil, nil
	}
	result := make(chan Result, 1)
	select {
	case r.queue <- request{callback, event, data, copy, result}:
		if event != "" {
			r.pending[event] = true
		}
		return result, nil
	default:
		return nil, errors.New("plugin callback queue is full")
	}
}

// Close cancels queued and running callbacks; bounded teardown continues on the VM owner.
func (r *Runtime) Close() { r.cancel() }

// Done closes after cleanup and VM destruction complete.
func (r *Runtime) Done() <-chan struct{} { return r.done }

// serve owns every VM access from candidate loading through callback execution and teardown.
func (r *Runtime) serve(directory string, c config.Config, actions []string, ready chan<- error) {
	defer close(r.done)
	e, closeFiles, err := load(r.ctx, directory, c, actions)
	if err != nil {
		ready <- err
		return
	}
	defer closeFiles()
	defer e.L.Close()
	defer func() {
		e.closing = true
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		e.L.SetContext(ctx)
		for i := len(e.cleanup) - 1; i >= 0 && ctx.Err() == nil; i-- {
			_ = e.L.CallByParam(lua.P{Fn: e.cleanup[i], NRet: 0, Protect: true})
		}
	}()
	r.manifest = e.manifest
	ready <- nil
	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.queue:
			result := e.invoke(r.ctx, req)
			r.mu.Lock()
			delete(r.pending, req.event)
			r.mu.Unlock()
			req.result <- result
		}
	}
}

// invoke discards the whole action batch when a callback fails or exceeds its deadline.
func (e *engine) invoke(parent context.Context, req request) Result {
	ctx, cancel := context.WithTimeout(parent, 250*time.Millisecond)
	defer cancel()
	e.L.SetContext(ctx)
	e.effects = nil
	e.manifest.Config = req.config
	if err := json.Unmarshal(req.data, &e.snapshot); err != nil {
		return Result{Err: err}
	}
	ids := []string{req.callback}
	if req.event != "" {
		ids = append([]string(nil), e.events[req.event]...)
	}
	for _, id := range ids {
		fn := e.functions[id]
		if fn == nil {
			return Result{Err: errors.New("plugin callback no longer exists")}
		}
		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, encodeValue(e.L, e.snapshot)); err != nil {
			return Result{Err: err}
		}
	}
	return Result{Effects: e.effects}
}
