package plugin

import (
	"errors"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

// installReadyReplay deduplicates lifecycle catchup within the current registration transaction.
func (e *engine) installReadyReplay(api *lua.LTable) {
	api.RawGetString("events").(*lua.LTable).RawSetString("replay_ready", e.L.NewFunction(func(L *lua.LState) int {
		e.guardModuleMetadata(L, "events.replay_ready")
		if e.pureHook {
			e.hookViolation = errors.New("ready replay is unavailable during pure hooks")
			L.RaiseError("%v", e.hookViolation)
		}
		if e.closing {
			L.RaiseError("ready replay is unavailable during teardown")
		}
		list := L.CheckTable(1)
		if list.Len() > 32 {
			L.RaiseError("events.replay_ready accepts up to 32 ready listeners")
		}
		ids, seen, count := make([]string, list.Len()), map[string]bool{}, 0
		list.ForEach(func(key, value lua.LValue) {
			index, numeric := key.(lua.LNumber)
			id, text := value.(lua.LString)
			if !numeric || index < 1 || index > lua.LNumber(len(ids)) || index != lua.LNumber(int(index)) || !text || seen[string(id)] || !slices.Contains(e.events["ready"], string(id)) {
				L.RaiseError("events.replay_ready requires unique current ready listeners in a consecutive array")
			}
			ids[int(index)-1], seen[string(id)] = string(id), true
			count++
		})
		if count != len(ids) {
			L.RaiseError("events.replay_ready requires a consecutive listener array")
		}
		if !e.readyStarted || len(ids) == 0 {
			return 0
		}

		for _, effect := range e.effects {
			if delivery, ok := effect.Data.(EventDelivery); ok && effect.Name == "event.dispatch" && effect.Argument == "ready" && delivery.replayReady {
				for _, id := range delivery.Listeners {
					delete(seen, id)
				}
			}
		}
		remaining := make([]string, 0, len(ids))
		for _, id := range ids {
			if seen[id] {
				remaining = append(remaining, id)
			}
		}
		if len(remaining) > 0 {
			e.emit(L, Effect{Name: "event.dispatch", Argument: "ready", Data: EventDelivery{Listeners: remaining, replayReady: true}})
		}
		return 0
	}))
}
