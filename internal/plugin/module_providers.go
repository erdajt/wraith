package plugin

import (
	"errors"
	"fmt"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

type moduleProvider struct {
	id, owner string
	resolve   *lua.LFunction
}

// addModuleProvider stages a bounded resolver under the current registration owner.
func (e *engine) addModuleProvider(resolve *lua.LFunction) (string, error) {
	if e.closing || e.owner == "" || resolve == nil {
		return "", errors.New("module provider requires an owner and resolver outside teardown")
	}
	if len(e.moduleProviders) >= 32 {
		return "", errors.New("module providers exceed 32")
	}
	e.serial++
	id := fmt.Sprintf("lua.module.provider.%d", e.serial)
	e.moduleProviders = append(e.moduleProviders, moduleProvider{id: id, owner: e.owner, resolve: resolve})
	e.dirty = !e.setup
	return id, nil
}

// removeModuleProvider removes only a registration belonging to the current owner.
func (e *engine) removeModuleProvider(id string) error {
	if e.closing {
		return errors.New("module providers cannot change during teardown")
	}
	index := slices.IndexFunc(e.moduleProviders, func(provider moduleProvider) bool { return provider.id == id })
	if index < 0 || e.moduleProviders[index].owner != e.owner {
		return fmt.Errorf("unknown or foreign module provider: %s", id)
	}
	e.moduleProviders = slices.Delete(e.moduleProviders, index, index+1)
	e.dirty = !e.setup
	return nil
}

// installModuleProviders exposes owned routing before user modules capture require.
func (e *engine) installModuleProviders(api *lua.LTable) {
	L := e.L
	native := e.nativeModules
	L.SetGlobal("require", L.NewFunction(func(L *lua.LState) int { return e.requireModule(L, native) }))
	modules := api.RawGetString("modules").(*lua.LTable)
	L.SetFuncs(modules, map[string]lua.LGFunction{
		"provider": func(L *lua.LState) int {
			e.registration(L)
			id, err := e.addModuleProvider(L.CheckFunction(1))
			if err != nil {
				L.RaiseError("modules.provider: %v", err)
			}
			L.Push(lua.LString(id))
			return 1
		},
		"remove_provider": func(L *lua.LState) int {
			e.registration(L)
			if err := e.removeModuleProvider(L.CheckString(1)); err != nil {
				L.RaiseError("modules.remove_provider: %v", err)
			}
			return 0
		},
	})
	api.RawGetString("capabilities").(*lua.LTable).RawSetString("module_providers", lua.LTrue)
	api.RawGetString("capabilities").(*lua.LTable).RawSetString("module_preparation", lua.LTrue)
}
