package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

type Command struct {
	Name, Description, Callback string
	NeedsArgument               bool
}

var ErrQueueFull = errors.New("plugin callback queue is full")

type Manifest struct {
	Providers   []ProviderDefinition
	Tools       []ToolDefinition
	Hooks       []HookDefinition
	Surface     []SurfaceMutation
	Resources   []string
	Config      config.Config
	Events      []string
	Commands    []Command
	Callbacks   []string
	Overridden  []string
	Diagnostics []string
}

type Effect struct {
	Name, Argument string
	Owner, Lease   string
	Invocation     string
	Config         *config.Config
	Data           any
}

type Result struct {
	Invocation string
	Effects    []Effect
	Err        error
	Manifest   *Manifest
	Revision   uint64
}

type request struct {
	callback, event string
	data            []byte
	config          config.Config
	reply           *completion
	revision        uint64
	resource        *ResourceEvent
	invocation      *InvocationStart
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
	invocations                             *invocationManager
	hooks                                   chan hookCall
	startupInstalled                        bool
	startupReady                            chan struct{}
	startupConsumed                         chan struct{}
	surfaces                                *SurfaceStore
	services                                *resourceManager
	discarded                               map[string]bool
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
	invocations        *invocationManager
	invocationID       string
	acceptedExtensions map[string]bool
	jsonCodec          jsonCodec
	extensionState
	pureHook      bool
	hookViolation error
	ownershipLayers
	schedulerState
	resourceState
	surfaceState
	surfaces       *SurfaceStore
	services       *resourceManager
	L              *lua.LState
	manifest       Manifest
	functions      map[string]*lua.LFunction
	events         map[string][]string
	cleanup        []cleanupHook
	disposing      []cleanupHook
	actions        map[string]bool
	builtins       map[string]bool
	effects        []Effect
	snapshot       any
	setup          bool
	closing        bool
	serial         uint64
	dirty          bool
	rollback       *registryState
	loaded         map[string]bool
	packages       map[string]packageRecord
	observerErrors []error
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
	r := &Runtime{
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
		startupReady: make(chan struct{}, 1), startupConsumed: make(chan struct{}),
		hooks: make(chan hookCall, 32),
		queue: make(chan request, 128), ack: make(chan publicationDecision, 1),
		notices: make(chan string, 64), discarded: map[string]bool{},
		pending: map[string]request{}, waiting: map[*completion]struct{}{},
	}
	r.services = newResourceManager(ctx)
	r.surfaces = newSurfaceStore(r.services.prefix)
	r.invocations = newInvocationManager(r.services.prefix, r.services.discard)
	ready := make(chan error, 1)
	go r.serve(directory, detached, actions, ready)
	go func() {
		<-ctx.Done()
		r.invocations.close()
		r.services.close()
		r.surfaces.close()
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
	return r.submit(callback, event, snapshot, c, nil, nil)
}

// submit queues detached state while preserving an optional managed resource identity.
func (r *Runtime) submit(callback, event string, snapshot any, c config.Config, resource *ResourceEvent, invocation *InvocationStart) (<-chan Result, error) {
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
	req := request{callback: callback, event: event, data: data, config: copy, reply: reply, revision: r.revision, resource: resource, invocation: invocation}
	if previous, ok := r.pending[event]; (event == "window_changed" || event == "session_changed" || event == "buffer_changed") && ok {
		previous.reply.resolve(Result{})
		delete(r.waiting, previous.reply)
		r.waiting[reply] = struct{}{}
		r.pending[event] = req
		return reply.channel, nil
	}
	select {
	case r.queue <- req:
		r.waiting[reply] = struct{}{}
		if event == "window_changed" || event == "session_changed" || event == "buffer_changed" {
			r.pending[event] = req
		}
		return reply.channel, nil
	default:
		return nil, ErrQueueFull
	}
}

// Close cancels queued and running callbacks; teardown continues on the VM owner.
func (r *Runtime) Close() {
	r.cancel()
	r.invocations.close()
	r.services.close()
	r.surfaces.close()
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
	e, err := load(r.ctx, directory, c, actions, runtimeHosts{r.services, r.surfaces, r.invocations})
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
		case call := <-r.hooks:
			result, err := e.transform(call.context, call.input)
			call.reply <- hookReply{result: result, err: err}
		case <-r.startupReady:
			e.manifest.Surface = nil
			e.surfaceStartupBytes = 0
			e.surfacePreview = nil
			close(r.startupConsumed)
		case req := <-r.queue:
			r.discardScheduled(e)
			r.mu.Lock()
			if r.startupInstalled {
				e.manifest.Surface = nil
				e.surfaceStartupBytes = 0
				e.surfacePreview = nil
			}
			if req.event == "window_changed" || req.event == "session_changed" || req.event == "buffer_changed" {
				req = r.pending[req.event]
				delete(r.pending, req.event)
			}
			scheduled := strings.HasPrefix(req.callback, "lua.schedule.")
			stale := req.event == "" && req.resource == nil && req.invocation == nil && !scheduled && req.revision != r.revision
			if req.event != "" || scheduled || req.resource != nil || req.invocation != nil {
				req.config.Keymaps = slices.Clone(r.manifest.Config.Keymaps)
			}
			r.mu.Unlock()
			result := Result{Err: errors.New("plugin registrations changed before callback dispatch")}
			if !stale {
				result = e.invoke(r.ctx, req)
				for _, err := range e.observerErrors {
					r.reportCleanup(err)
				}
			}
			r.mu.Lock()
			if err := r.ctx.Err(); err != nil {
				result = Result{Err: err}
			}
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
			r.discardScheduled(e)
		}
	}
}

// invoke stages command or isolated observer changes for host publication.
func (e *engine) invoke(parent context.Context, req request) (result Result) {
	e.observerErrors = nil
	e.invocationID = ""
	defer func() { result.Invocation = e.invocationID; e.invocationID = "" }()
	e.surfacePreview = nil
	e.surfaceReserved = e.surfaces.pendingReservations(e.surfaceReserved)
	e.pruneResources()
	var deferred scheduledCall
	if strings.HasPrefix(req.callback, "lua.schedule.") {
		var exists bool
		deferred, exists = e.scheduled[req.callback]
		if !exists {
			return Result{}
		}
		delete(e.scheduled, req.callback)
		e.invocationID = deferred.invocation
	}
	var resource resourceCallback
	if req.resource != nil {
		var exists bool
		resource, exists = e.resources[req.resource.ID]
		if !exists || !e.services.valid(*req.resource) || e.delivered[req.resource.ID] >= req.resource.Sequence {
			return Result{}
		}
		e.delivered[req.resource.ID] = req.resource.Sequence
		e.invocationID = resource.invocation
		if resource.function == nil {
			return Result{}
		}
	}
	if req.invocation != nil {
		e.invocationID = req.invocation.ID
	}
	if e.invocationID != "" && !e.invocations.valid(e.invocationID) {
		return Result{Err: errors.New("invocation is no longer active")}
	}
	before := e.captureRegistry()
	e.dirty = false
	defer func() {
		if result.Err != nil {
			if e.invocationID != "" {
				e.invocations.finish(e.invocationID, nil, result.Err)
			}
			hooks := e.restoreRegistry(before)
			if req.resource != nil {
				callback := e.resources[req.resource.ID]
				callback.function = nil
				e.resources[req.resource.ID] = callback
				result.Err = errors.Join(result.Err, e.services.failConsumer(req.resource.ID, resource.owner, result.Err))
			}
			result.Err = errors.Join(result.Err, e.disposeHooks(hooks))
		}
	}()
	e.L.SetContext(parent)
	e.effects = nil
	e.syncOwnerBindings(req.config.Keymaps, "config")
	e.manifest.Config = req.config
	if err := json.Unmarshal(req.data, &e.snapshot); err != nil {
		return Result{Err: err}
	}
	if req.event != "" {
		if err := e.invokeObservers(parent, req.event); err != nil {
			return Result{Err: err}
		}
	} else {
		fn := e.functions[req.callback]
		owner := e.callbackOwners[req.callback]
		if deferred.function != nil {
			fn, owner = deferred.function, deferred.owner
		}
		if req.resource != nil {
			fn, owner = resource.function, resource.owner
		}
		if fn == nil {
			return Result{Err: errors.New("plugin callback no longer exists")}
		}
		ctx, cancel := context.WithTimeout(parent, 250*time.Millisecond)
		defer cancel()
		e.L.SetContext(ctx)
		e.owner = owner
		argument, err := e.callbackContext(req.resource)
		if err == nil {
			argument, err = e.invocationContext(argument, req.invocation)
		}
		if err != nil {
			return Result{Err: err}
		}
		if err := e.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, argument); err != nil {
			return Result{Err: err}
		}
	}
	e.owner = "config"
	if e.dirty {
		e.refreshManifest()
		if err := e.validateRegistry(); err != nil {
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
