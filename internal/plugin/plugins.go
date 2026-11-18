package plugin

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

// installPlugins loads setup modules with independent registration and cleanup transactions.
func (e *engine) installPlugins(api *lua.LTable) {
	L := e.L
	e.loaded = map[string]bool{}
	e.packages = map[string]packageRecord{}
	api.RawSetString("plugins", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"list": func(L *lua.LState) int {
			names := L.NewTable()
			for _, name := range slices.Sorted(maps.Keys(e.loaded)) {
				names.Append(lua.LString(name))
			}
			L.Push(names)
			return 1
		},
		"unload": func(L *lua.LState) int {
			e.registration(L)
			if err := e.unloadPlugin(L.CheckString(1)); err != nil {
				L.RaiseError("%v", err)
			}
			return 0
		},
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
	parentOwner := e.owner
	e.serial++
	owner := fmt.Sprintf("plugin:%s:%d", name, e.serial)
	e.owner = owner
	e.loaded[name] = true
	e.packages[name] = packageRecord{owner: owner}
	defer func() { e.owner = parentOwner }()
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
	for parent, record := range e.packages {
		if record.owner == parentOwner {
			record.dependencies = append(record.dependencies, owner)
			e.packages[parent] = record
		}
	}
	var owned []cleanupHook
	e.cleanup = slices.DeleteFunc(e.cleanup, func(hook cleanupHook) bool {
		if hook.owner != owner {
			return false
		}
		owned = append(owned, hook)
		return true
	})
	e.cleanup = append(e.cleanup, owned...)
	e.dirty = !e.setup
	return nil
}
