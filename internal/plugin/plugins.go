package plugin

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// installPlugins loads setup modules with independent registration and cleanup transactions.
func (e *engine) installPlugins(api *lua.LTable) {
	L := e.L
	e.loaded = map[string]bool{}
	api.RawSetString("plugins", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"load": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			if e.loaded[name] || len(e.loaded) >= 128 {
				L.RaiseError("plugin already loaded or plugin limit reached: %s", name)
			}
			if err := e.loadPlugin(name, L.OptTable(2, L.NewTable())); err != nil {
				L.RaiseError("plugin %s: %v", name, err)
			}
			return 0
		},
	}))
}

// loadPlugin restores partial registrations and releases new resources even when Lua catches failure.
func (e *engine) loadPlugin(name string, options *lua.LTable) (err error) {
	before := e.captureRegistry()
	defer func() {
		if err != nil {
			hooks := e.restoreRegistry(before)
			err = errors.Join(err, e.disposeHooks(hooks))
		}
	}()
	L := e.L
	if err := L.CallByParam(lua.P{Fn: L.GetGlobal("require"), NRet: 1, Protect: true}, lua.LString(name)); err != nil {
		return err
	}
	module, ok := L.Get(-1).(*lua.LTable)
	L.Pop(1)
	if !ok {
		return errors.New("module must return a table with setup")
	}
	setup, ok := module.RawGetString("setup").(*lua.LFunction)
	if !ok {
		return errors.New("module has no setup function")
	}
	if value := module.RawGetString("teardown"); value != lua.LNil {
		teardown, ok := value.(*lua.LFunction)
		if !ok {
			return errors.New("module teardown must be a function")
		}
		e.addCleanup(teardown)
	}
	if err := L.CallByParam(lua.P{Fn: setup, NRet: 1, Protect: true}, options); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	if cleanup, ok := L.Get(-1).(*lua.LFunction); ok {
		e.addCleanup(cleanup)
	}
	L.Pop(1)
	e.loaded[name] = true
	return nil
}
