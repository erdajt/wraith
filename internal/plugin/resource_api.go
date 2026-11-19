package plugin

import (
	"errors"
	"maps"
	"slices"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

type resourceCallback struct {
	function   *lua.LFunction
	owner      string
	invocation string
}

type resourceState struct {
	resources map[string]resourceCallback
	released  []string
	delivered map[string]uint64
}

// cloneResources preserves reservation ownership across callback and package transactions.
func (e *engine) cloneResources() resourceState {
	return resourceState{maps.Clone(e.resources), slices.Clone(e.released), maps.Clone(e.delivered)}
}

// reserveResource stages an owned worker and its callback without performing external work.
func (e *engine) reserveResource(L *lua.LState, kind string, callback *lua.LFunction, work resourceWork) string {
	if e.closing {
		L.RaiseError("resource creation is disabled during teardown")
	}
	if !e.setup && e.effectCount("resource.activate") >= 64 {
		L.RaiseError("plugin action batch exceeds its limit")
	}
	id, err := e.services.reserve(e.owner, kind, work)
	if err != nil {
		L.RaiseError("%v", err)
	}
	if e.invocationID != "" {
		if err := e.invocations.attach(e.invocationID, id); err != nil {
			L.RaiseError("%v", err)
		}
	}
	e.resources[id] = resourceCallback{function: callback, owner: e.owner, invocation: e.invocationID}
	if e.setup {
		e.manifest.Resources = append(e.manifest.Resources, id)
	} else {
		e.emit(L, Effect{Name: "resource.activate", Argument: id})
	}
	return id
}

// installResources exposes owner-scoped status and cancellation with ordered handle release.
func (e *engine) installResources(api *lua.LTable) {
	e.resources = map[string]resourceCallback{}
	e.delivered = map[string]uint64{}
	api.RawSetString("resource", e.L.SetFuncs(e.L.NewTable(), map[string]lua.LGFunction{
		"status": func(L *lua.LState) int {
			status, err := e.services.Status(L.CheckString(1), e.owner)
			if err != nil {
				L.RaiseError("%v", err)
			}
			value, err := encodeResourceValue(L, status)
			if err != nil {
				L.RaiseError("%v", err)
			}
			L.Push(value)
			return 1
		},
		"cancel": func(L *lua.LState) int {
			if err := e.services.Cancel(L.CheckString(1), e.owner); err != nil {
				L.RaiseError("%v", err)
			}
			return 0
		},
		"release": func(L *lua.LState) int {
			id := L.CheckString(1)
			if _, err := e.services.Status(id, e.owner); err != nil {
				L.RaiseError("%v", err)
			}
			if e.setup {
				e.services.discard([]string{id})
				delete(e.resources, id)
				e.manifest.Resources = slices.DeleteFunc(e.manifest.Resources, func(resource string) bool { return resource == id })
			} else {
				e.emit(L, Effect{Name: "resource.release", Argument: id})
			}
			return 0
		},
	}))
}

// Events exposes detached managed notifications without entering the Lua actor.
func (r *Runtime) Events() <-chan ResourceEvent { return r.services.events }

// Activate starts only the staged resources the host has accepted.
func (r *Runtime) Activate(ids []string) error {
	for _, id := range ids {
		if err := r.services.activate(id); err != nil {
			return err
		}
	}
	return nil
}

// DiscardResources invalidates abandoned reservations and stops active work independently of Lua.
func (r *Runtime) DiscardResources(ids []string) { r.services.discard(ids) }

// ResourceValid accepts retained terminal handles and rejects released or foreign-generation events.
func (r *Runtime) ResourceValid(event ResourceEvent) bool { return r.services.valid(event) }

// SubmitResource keeps exact worker bytes separate from the JSON workspace snapshot.
func (r *Runtime) SubmitResource(event ResourceEvent, snapshot any, c config.Config) (<-chan Result, error) {
	if !r.ResourceValid(event) {
		return nil, errors.New("resource event is no longer valid")
	}
	detached, err := cloneResourceValue(event.Data)
	if err != nil {
		return nil, err
	}
	event.Data = detached.(map[string]any)
	return r.submit("", "", snapshot, c, &event, nil)
}

// callbackContext attaches a copied worker event to current workspace state on the VM owner.
func (e *engine) callbackContext(event *ResourceEvent) (lua.LValue, error) {
	value := encodeValue(e.L, e.snapshot)
	if event == nil {
		return value, nil
	}
	table, ok := value.(*lua.LTable)
	if !ok {
		table = e.L.NewTable()
	}
	data, err := encodeResourceValue(e.L, event.Data)
	if err != nil {
		return nil, err
	}
	metadata, err := encodeResourceValue(e.L, map[string]any{"id": event.ID, "revision": event.Revision, "sequence": event.Sequence, "kind": event.Kind, "final": event.Final})
	if err != nil {
		return nil, err
	}
	table.RawSetString("data", data)
	table.RawSetString("resource", metadata)
	return table, nil
}

// pruneResources drops private Lua callbacks after host release without waiting for another publication.
func (e *engine) pruneResources() {
	e.services.mu.Lock()
	defer e.services.mu.Unlock()
	for id := range e.resources {
		if e.services.records[id] == nil {
			delete(e.resources, id)
			delete(e.delivered, id)
		}
	}
	e.manifest.Resources = slices.DeleteFunc(e.manifest.Resources, func(id string) bool { return e.services.records[id] == nil })
}
