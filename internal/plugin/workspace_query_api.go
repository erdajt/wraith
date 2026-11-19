package plugin

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installWorkspaceQueries stages bounded host reads with owned once-only callbacks.
func (e *engine) installWorkspaceQueries(api *lua.LTable) {
	L := e.L
	for kind, fields := range workspaceFields {
		namespace, name, _ := strings.Cut(kind, ".")
		table, ok := api.RawGetString(namespace).(*lua.LTable)
		if !ok {
			table = L.NewTable()
			api.RawSetString(namespace, table)
		}
		table.RawSetString(name, L.NewFunction(func(L *lua.LState) int {
			if e.closing {
				L.RaiseError("workspace queries are disabled during teardown")
			}
			options := L.CheckTable(1)
			options.ForEach(func(key, value lua.LValue) {
				if key.Type() != lua.LTString || !strings.Contains(" "+fields+" ", " "+key.String()+" ") {
					L.RaiseError("unknown %s option: %s", kind, key)
				}
			})
			query := WorkspaceQuery{Kind: kind}
			decodeTypedValue(L, options, &query.Options)
			if err := query.Validate(); err != nil {
				L.RaiseError("%s: %v", kind, err)
			}
			callback := L.CheckFunction(2)
			if len(e.scheduled) >= 128 {
				L.RaiseError("pending callback limit reached")
			}
			e.serial++
			query.ID = fmt.Sprintf("lua.schedule.%d", e.serial)
			query.Owner, query.Lease = e.owner, e.surfaceLease()
			query.Invocation = e.invocationID
			e.scheduled[query.ID] = scheduledCall{function: callback, owner: e.owner, invocation: e.invocationID}
			published := false
			defer func() {
				if !published {
					delete(e.scheduled, query.ID)
				}
			}()
			effect := Effect{Name: "workspace.query", Argument: query.ID, Data: query}
			if e.setup {
				if err := L.CallByParam(lua.P{Fn: api.RawGetString("schedule"), NRet: 0, Protect: true}, L.NewFunction(func(L *lua.LState) int {
					e.emit(L, effect)
					return 0
				})); err != nil {
					L.RaiseError("%s: %v", kind, err)
				}
			} else {
				e.emit(L, effect)
			}
			published = true
			L.Push(lua.LString(query.ID))
			return 1
		}))
	}
	workspace := L.NewTable()
	workspace.RawSetString("cancel", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		if !strings.HasPrefix(id, "lua.schedule.") || len(id) > 64 {
			L.RaiseError("invalid workspace query id")
		}
		e.emit(L, Effect{Name: "workspace.cancel", Argument: id})
		return 0
	}))
	api.RawSetString("workspace", workspace)
}
