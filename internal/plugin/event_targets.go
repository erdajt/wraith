package plugin

import (
	"errors"
	"slices"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

type EventDelivery struct {
	Listeners   []string
	Data        any
	replayReady bool
}

// validEventName admits native notifications and namespaced custom events.
func validEventName(name string) bool {
	return slices.Contains([]string{"ready", "session_changed", "window_changed", "buffer_changed", "response_finished", "jobs_changed", "terminals_changed", "environment_changed"}, name) || customEventName.MatchString(name) && len(name) <= 128
}

// installEventTargets exposes explicit subscriber snapshots without broadcasting native events.
func (e *engine) installEventTargets(api *lua.LTable) {
	L := e.L
	events := api.RawGetString("events").(*lua.LTable)
	events.RawSetString("listeners", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if !validEventName(name) {
			L.RaiseError("unknown event: %s", name)
		}
		result := L.NewTable()
		for _, id := range e.events[name] {
			result.Append(lua.LString(id))
		}
		L.Push(result)
		return 1
	}))
	events.RawSetString("dispatch", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if !validEventName(name) {
			L.RaiseError("unknown event: %s", name)
		}
		options := L.CheckTable(3)
		options.ForEach(func(key, _ lua.LValue) {
			if key.String() != "listeners" {
				L.RaiseError("events.dispatch: unknown option %s", key)
			}
		})
		list, ok := options.RawGetString("listeners").(*lua.LTable)
		if !ok || list.Len() > 32 {
			L.RaiseError("events.dispatch requires up to 32 explicit listeners")
		}
		ids, count := make([]string, list.Len()), 0
		seen := map[string]bool{}
		list.ForEach(func(key, value lua.LValue) {
			index, valid := key.(lua.LNumber)
			id, text := value.(lua.LString)
			if !valid || index < 1 || index > lua.LNumber(len(ids)) || index != lua.LNumber(int(index)) || !text || seen[string(id)] || !slices.Contains(e.events[name], string(id)) {
				L.RaiseError("events.dispatch requires unique listeners subscribed to %s", name)
			}
			ids[int(index)-1], seen[string(id)] = string(id), true
			count++
		})
		if count != len(ids) {
			L.RaiseError("events.dispatch requires a consecutive listener array")
		}
		data, err := decodeValue(L.Get(2))
		if err != nil {
			L.RaiseError("events.dispatch: %v", err)
		}
		e.emit(L, Effect{Name: "event.dispatch", Argument: name, Data: EventDelivery{Listeners: ids, Data: data}})
		return 0
	}))
}

// SubmitEvent queues an explicit subscriber set without coalescing or adding later listeners.
func (r *Runtime) SubmitEvent(event string, listeners []string, snapshot any, c config.Config, invocation string) (<-chan Result, error) {
	if !validEventName(event) || len(listeners) > 32 {
		return nil, errors.New("invalid targeted event")
	}
	if invocation != "" && !r.invocations.valid(invocation) {
		return nil, errors.New("invocation is no longer active")
	}
	return r.submit("", event, snapshot, c, nil, nil, invocation, listeners)
}
