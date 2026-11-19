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
		"call": func(L *lua.LState) int {
			if e.closing {
				L.RaiseError("package calls are disabled during teardown")
			}
			if L.GetTop() > 18 {
				L.RaiseError("plugins.call accepts at most 16 arguments")
			}
			args := make([]lua.LValue, max(0, L.GetTop()-2))
			for i := range args {
				args[i] = L.Get(i + 3)
			}
			value, err := e.callPackage(L.CheckString(1), L.CheckString(2), args)
			if err != nil {
				L.RaiseError("plugins.call: %v", err)
			}
			L.Push(value)
			return 1
		},
		"depend": func(L *lua.LState) int {
			e.registration(L)
			if err := e.dependOnPackage(e.owner, L.CheckString(1)); err != nil {
				L.RaiseError("plugins.depend: %v", err)
			}
			return 0
		},
		"load": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			if e.loaded[name] || len(e.loaded) >= 128 {
				L.RaiseError("plugin already loaded or plugin limit reached: %s", name)
			}
			dependency := true
			if options := L.OptTable(3, nil); options != nil {
				options.ForEach(func(key, value lua.LValue) {
					flag, ok := value.(lua.LBool)
					if key.String() != "dependency" || !ok {
						L.RaiseError("plugin load options require a boolean dependency")
					}
					dependency = bool(flag)
				})
			}
			if err := e.loadPlugin(name, L.OptTable(2, L.NewTable()), dependency); err != nil {
				L.RaiseError("plugin %s: %v", name, err)
			}
			return 0
		},
	}))
}

// loadPlugin restores partial registrations and releases new resources even when Lua catches failure.
func (e *engine) loadPlugin(name string, options *lua.LTable, dependency bool) (err error) {
	before := e.captureRegistry()
	parentOwner := e.owner
	e.serial++
	owner := fmt.Sprintf("plugin:%s:%d", name, e.serial)
	e.owner = owner
	e.loaded[name] = true
	e.packages[name] = packageRecord{owner: owner, loading: true}
	defer func() { e.owner = parentOwner }()
	defer func() {
		if err != nil {
			disposal := e.restoreRegistry(before)
			err = errors.Join(err, e.disposeRegistry(disposal))
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
	record := e.packages[name]
	record.module = module
	e.packages[name] = record
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
	record = e.packages[name]
	record.loading = false
	e.packages[name] = record
	if dependency {
		for _, parent := range e.packages {
			if parent.owner == parentOwner {
				if err := e.dependOnPackage(parentOwner, name); err != nil {
					return err
				}
				break
			}
		}
	}
	e.dirty = !e.setup
	return nil
}
