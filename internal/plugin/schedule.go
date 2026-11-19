package plugin

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type scheduledCall struct {
	function *lua.LFunction
	owner    string
}

type schedulerState struct {
	scheduled  map[string]scheduledCall
	startup    []string
	dispatcher string
}

// cloneScheduler preserves pending work when a callback or package setup is rejected.
func (e *engine) cloneScheduler() schedulerState {
	return schedulerState{maps.Clone(e.scheduled), slices.Clone(e.startup), e.dispatcher}
}

// installSchedule queues owned callbacks after the current host action batch finishes.
func (e *engine) installSchedule(api *lua.LTable) {
	e.scheduled = map[string]scheduledCall{}
	api.RawSetString("schedule", e.L.NewFunction(func(L *lua.LState) int {
		function := L.CheckFunction(1)
		if e.closing {
			L.RaiseError("scheduling is disabled during teardown")
		}
		if len(e.scheduled) >= 128 {
			L.RaiseError("scheduled callback limit reached")
		}
		if e.setup && e.dispatcher == "" {
			if len(e.events["ready"]) >= 32 {
				L.RaiseError("ready observer limit reached")
			}
			owner := e.owner
			e.owner = "config"
			e.dispatcher = e.callback(L, L.NewFunction(func(L *lua.LState) int {
				for _, id := range e.startup {
					if _, ok := e.scheduled[id]; ok {
						e.emit(L, Effect{Name: "schedule.callback", Argument: id})
					}
				}
				e.startup = nil
				e.events["ready"] = slices.DeleteFunc(e.events["ready"], func(id string) bool { return id == e.dispatcher })
				e.dirty = true
				return 0
			}))
			e.owner = owner
			e.events["ready"] = append(e.events["ready"], e.dispatcher)
		}
		if e.setup && len(e.startup) >= 64 {
			L.RaiseError("startup schedule limit reached")
		}
		if !e.setup && e.effectCount("schedule.callback") >= 64 {
			L.RaiseError("plugin action batch exceeds its limit")
		}
		e.serial++
		id := fmt.Sprintf("lua.schedule.%d", e.serial)
		e.scheduled[id] = scheduledCall{function: function, owner: e.owner}
		if e.setup {
			e.startup = append(e.startup, id)
		} else {
			e.emit(L, Effect{Name: "schedule.callback", Argument: id})
		}
		return 0
	}))
}

// DiscardScheduled releases abandoned action-chain work before the next VM callback.
func (r *Runtime) DiscardScheduled(ids []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctx.Err() != nil {
		return
	}
	for _, id := range ids {
		if strings.HasPrefix(id, "lua.schedule.") && len(id) <= 64 {
			r.discarded[id] = true
		}
	}
}

// discardScheduled applies host cancellations without sharing Lua values across goroutines.
func (r *Runtime) discardScheduled(e *engine) {
	r.mu.Lock()
	ids := r.discarded
	r.discarded = map[string]bool{}
	r.mu.Unlock()
	for id := range ids {
		delete(e.scheduled, id)
		if e.rollback != nil {
			delete(e.rollback.scheduled, id)
		}
	}
}
