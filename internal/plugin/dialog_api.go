package plugin

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// installDialogs stages modal requests with private callbacks consumed once on the VM owner.
func (e *engine) installDialogs(api *lua.LTable) {
	L := e.L
	ui := L.NewTable()
	for _, kind := range []string{"select", "input"} {
		ui.RawSetString(kind, L.NewFunction(func(L *lua.LState) int {
			if e.closing {
				L.RaiseError("dialogs are disabled during teardown")
			}
			dialog, err := parseDialog(L, kind)
			if err != nil {
				L.RaiseError("ui.%s: %v", kind, err)
			}
			index := 2
			if kind == "select" {
				index = 3
			}
			callback := L.CheckFunction(index)
			if len(e.scheduled) >= 128 {
				L.RaiseError("pending callback limit reached")
			}
			e.serial++
			dialog.ID = fmt.Sprintf("lua.schedule.%d", e.serial)
			dialog.Owner, dialog.Lease = e.owner, e.surfaceLease()
			e.scheduled[dialog.ID] = scheduledCall{function: callback, owner: e.owner}
			published := false
			defer func() {
				if !published {
					delete(e.scheduled, dialog.ID)
				}
			}()
			effect := Effect{Name: "dialog.open", Argument: dialog.ID, Data: dialog}
			if e.setup {
				if err := L.CallByParam(lua.P{Fn: api.RawGetString("schedule"), NRet: 0, Protect: true}, L.NewFunction(func(L *lua.LState) int {
					e.emit(L, effect)
					return 0
				})); err != nil {
					L.RaiseError("ui.%s: %v", kind, err)
				}
			} else {
				e.emit(L, effect)
			}
			published = true
			L.Push(lua.LString(dialog.ID))
			return 1
		}))
	}
	api.RawSetString("ui", ui)
}
