package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

type Command struct {
	Name, Description, Callback string
	NeedsArgument               bool
}

type Manifest struct {
	Config      config.Config
	Events      []string
	Commands    []Command
	Callbacks   []string
	Overridden  []string
	Diagnostics []string
}

type Effect struct {
	Name, Argument string
	Config         *config.Config
	Data           any
}

type Result struct {
	Effects  []Effect
	Err      error
	Manifest *Manifest
	Revision uint64
}

type request struct {
	callback, event string
	data            []byte
	config          config.Config
	reply           *completion
	revision        uint64
}

type completion struct {
	channel chan Result
	once    sync.Once
}

// resolve releases a caller once, including cancellation during a blocking native call.
func (c *completion) resolve(result Result) {
	c.once.Do(func() { c.channel <- result })
}

type Runtime struct {
	diagnostics                             []string
	notices                                 chan string
	queue                                   chan request
	done                                    chan struct{}
	ctx                                     context.Context
	cancel                                  context.CancelFunc
	mu                                      sync.Mutex
	pending                                 map[string]request
	waiting                                 map[*completion]struct{}
	manifest                                Manifest
	revision, nextRevision, pendingRevision uint64
	ack                                     chan publicationDecision
}

type engine struct {
	ownershipLayers
	L         *lua.LState
	manifest  Manifest
	functions map[string]*lua.LFunction
	events    map[string][]string
	cleanup   []cleanupHook
	disposing []cleanupHook
	actions   map[string]bool
	builtins  map[string]bool
	effects   []Effect
	snapshot  any
	setup     bool
	closing   bool
	serial    uint64
	dirty     bool
	rollback  *registryState
	loaded    map[string]bool
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
	r := &Runtime{notices: make(chan string, 64), queue: make(chan request, 128), ack: make(chan publicationDecision, 1), done: make(chan struct{}), ctx: ctx, cancel: cancel, pending: map[string]request{}, waiting: map[*completion]struct{}{}}
	ready := make(chan error, 1)
	go r.serve(directory, detached, actions, ready)
	go func() {
		<-ctx.Done()
		r.cancelWaiters()
	}()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneManifest(r.manifest)
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
	reply := &completion{channel: make(chan Result, 1)}
	req := request{callback: callback, event: event, data: data, config: copy, reply: reply, revision: r.revision}
	if previous, ok := r.pending[event]; (event == "window_changed" || event == "session_changed") && ok {
		previous.reply.resolve(Result{})
		delete(r.waiting, previous.reply)
		r.waiting[reply] = struct{}{}
		r.pending[event] = req
		return reply.channel, nil
	}
	select {
	case r.queue <- req:
		r.waiting[reply] = struct{}{}
		if event == "window_changed" || event == "session_changed" {
			r.pending[event] = req
		}
		return reply.channel, nil
	default:
		return nil, errors.New("plugin callback queue is full")
	}
}

// Close cancels queued and running callbacks; bounded teardown continues on the VM owner.
func (r *Runtime) Close() {
	r.cancel()
	r.cancelWaiters()
}

// Done closes after cleanup and VM destruction complete.
func (r *Runtime) Done() <-chan struct{} { return r.done }

// serve owns every VM access from candidate loading through callback execution and teardown.
func (r *Runtime) serve(directory string, c config.Config, actions []string, ready chan<- error) {
	defer close(r.done)
	defer close(r.notices)
	defer r.cancel()
	defer r.cancelWaiters()
	e, err := load(r.ctx, directory, c, actions)
	if err != nil {
		ready <- err
		return
	}
	defer e.L.Close()
	defer func() { r.reportCleanup(e.teardown(0)) }()

	r.mu.Lock()
	r.manifest = cloneManifest(e.manifest)
	r.diagnostics = slices.Clone(e.manifest.Diagnostics)
	r.mu.Unlock()
	ready <- nil
	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.queue:
			r.mu.Lock()
			if req.event == "window_changed" || req.event == "session_changed" {
				req = r.pending[req.event]
				delete(r.pending, req.event)
			}
			stale := req.event == "" && req.revision != r.revision
			if req.event != "" {
				req.config.Keymaps = slices.Clone(r.manifest.Config.Keymaps)
			}
			r.mu.Unlock()
			result := Result{Err: errors.New("plugin registrations changed before callback dispatch")}
			if !stale {
				result = e.invoke(r.ctx, req)
			}
			r.mu.Lock()
			if result.Manifest != nil {
				r.nextRevision++
				result.Revision, r.pendingRevision = r.nextRevision, r.nextRevision
			}
			req.reply.resolve(result)
			delete(r.waiting, req.reply)
			r.mu.Unlock()
			if result.Manifest != nil && !r.publish(e, result.Revision) {
				return
			}
		}
	}
}

// invoke discards the whole action batch when a callback fails or exceeds its deadline.
func (e *engine) invoke(parent context.Context, req request) (result Result) {
	before := e.captureRegistry()
	e.dirty = false
	defer func() {
		if result.Err != nil {
			hooks := e.restoreRegistry(before)
			result.Err = errors.Join(result.Err, e.disposeHooks(hooks))
		}
	}()
	ctx, cancel := context.WithTimeout(parent, 250*time.Millisecond)
	defer cancel()
	e.L.SetContext(ctx)
	e.effects = nil
	e.syncOwnerBindings(req.config.Keymaps, "config")
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
		e.owner = e.callbackOwners[id]
		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, encodeValue(e.L, e.snapshot)); err != nil {
			return Result{Err: err}
		}
	}
	e.owner = "config"
	if e.dirty {
		e.refreshManifest()
		if err := e.manifest.Config.Validate(); err != nil {
			return Result{Err: err}
		}
		for i := range e.effects {
			if e.effects[i].Config != nil {
				e.effects[i].Config.Keymaps = slices.Clone(e.manifest.Config.Keymaps)
			}
		}
		e.rollback = &before
		manifest := cloneManifest(e.manifest)
		return Result{Effects: e.effects, Manifest: &manifest}
	}
	return Result{Effects: e.effects}
}

// cancelWaiters releases queued and running callers without waiting for native Lua calls.
func (r *Runtime) cancelWaiters() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for reply := range r.waiting {
		reply.resolve(Result{Err: context.Canceled})
		delete(r.waiting, reply)
	}
}
