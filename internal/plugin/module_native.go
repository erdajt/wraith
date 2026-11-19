package plugin

import (
	"errors"
	"fmt"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

type nativeModules struct {
	require              *lua.LFunction
	pkg, loaded, preload *lua.LTable
}

// captureNativeModules retains native loading state before user configuration can replace it.
func captureNativeModules(L *lua.LState) (nativeModules, error) {
	native := nativeModules{}
	native.require, _ = L.GetGlobal("require").(*lua.LFunction)
	native.pkg, _ = L.GetGlobal("package").(*lua.LTable)
	if native.pkg != nil {
		native.loaded, _ = native.pkg.RawGetString("loaded").(*lua.LTable)
		native.preload, _ = native.pkg.RawGetString("preload").(*lua.LTable)
	}
	return native, native.validate(L)
}

// validate rejects replaced or intercepted caches without changing user lua state.
func (native nativeModules) validate(L *lua.LState) error {
	if native.require == nil || native.pkg == nil || native.loaded == nil || native.preload == nil ||
		L.GetGlobal("package") != native.pkg || native.pkg.RawGetString("loaded") != native.loaded ||
		native.pkg.RawGetString("preload") != native.preload ||
		L.Get(lua.RegistryIndex).(*lua.LTable).RawGetString("_LOADED") != native.loaded ||
		L.GetMetatable(native.loaded) != lua.LNil {
		return errors.New("managed module tables changed or cache has a metatable; use /reload")
	}
	return nil
}

// validateModuleTarget keeps the resolver and provider generation fixed across lua callbacks.
func (e *engine) validateModuleTarget(L *lua.LState, resolution *moduleResolution, consumer, owner string, native nativeModules) error {
	if e.closing {
		return errors.New("managed module execution is unavailable during cleanup")
	}
	if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if err := native.validate(L); err != nil {
		return err
	}
	if resolution == nil || resolution.check == nil || !slices.Contains(e.moduleProviders, resolution.provider) {
		return errors.New("module resolver registration changed")
	}
	record, ok := e.packages[resolution.plugin]
	if !ok || owner == "" || record.owner != owner || record.loading && consumer != owner {
		return fmt.Errorf("module provider is unavailable or still loading: %s", resolution.plugin)
	}
	return nil
}

// invokeOwnedModule delegates cache semantics to native require under the selected provider owner.
func (e *engine) invokeOwnedModule(L *lua.LState, name string, resolution *moduleResolution, consumer, owner string, native nativeModules) (value lua.LValue, err error) {
	if err := e.validateModuleTarget(L, resolution, consumer, owner, native); err != nil {
		return lua.LNil, err
	}
	if e.moduleInvocations >= 32 {
		return lua.LNil, errors.New("managed module invocation nesting exceeds 32")
	}
	release, err := e.claimModule(name)
	if err != nil {
		return lua.LNil, err
	}
	defer release()
	state, originalOwner, calls, top := e.L, e.owner, len(e.packageCalls), L.GetTop()
	e.L, e.owner = L, owner
	e.moduleInvocations++
	e.packageCalls = append(e.packageCalls, consumer, resolution.provider.owner, owner)
	before := e.captureRegistry()
	defer func() {
		if err != nil {
			value = lua.LNil
			err = errors.Join(err, e.disposeRegistry(e.restoreRegistry(before)))
		}
		L.SetTop(top)
		e.L, e.owner = state, originalOwner
		e.packageCalls = e.packageCalls[:calls]
		e.moduleInvocations--
	}()
	if L.IsModuleLoading(name) {
		err = L.CallByParam(lua.P{Fn: native.require, NRet: 1, Protect: true}, lua.LString(name))
		return lua.LNil, err
	}
	initial := native.loaded.RawGetString(name)
	verification := moduleVerification{phase: "before", executed: !lua.LVAsBool(initial)}
	verification.proof, err = e.checkModuleResolution(L, name, resolution, verification)
	if err == nil {
		err = e.validateModuleTarget(L, resolution, consumer, owner, native)
	}
	if native.validate(L) != nil || native.loaded.RawGetString(name) != initial {
		err = errors.Join(err, errors.New("managed module cache changed before execution; use /reload"))
		err = errors.Join(err, e.poisonModule(name, err.Error()))
	}
	if err != nil {
		return lua.LNil, err
	}
	executed := verification.executed
	if err = L.CallByParam(lua.P{Fn: native.require, NRet: 1, Protect: true}, lua.LString(name)); err != nil {
		current := native.loaded.RawGetString(name)
		clean := current == initial
		if executed {
			clean = current == lua.LNil || initial == lua.LFalse && current == initial
		}
		if native.validate(L) != nil || !clean {
			err = errors.Join(err, e.poisonModule(name, err.Error()))
		}
		return lua.LNil, err
	}
	value = L.Get(-1)
	current := native.loaded.RawGetString(name)
	err = e.validateModuleTarget(L, resolution, consumer, owner, native)
	if err == nil {
		verification.phase = "after"
		_, err = e.checkModuleResolution(L, name, resolution, verification)
	}
	if err == nil {
		err = e.validateModuleTarget(L, resolution, consumer, owner, native)
	}
	if err == nil && (native.loaded.RawGetString(name) != current || value != current) {
		err = errors.New("managed module cache changed during verification; use /reload")
	}
	if err != nil {
		if executed || native.validate(L) != nil || native.loaded.RawGetString(name) != initial {
			err = errors.Join(err, e.poisonModule(name, err.Error()))
		}
		return lua.LNil, err
	}
	if executed {
		if err = e.recordModuleExecution(name, owner, native.loaded); err != nil {
			err = errors.Join(err, e.poisonModule(name, err.Error()))
			return lua.LNil, err
		}
	}
	if consumer != owner {
		for _, record := range e.packages {
			if record.owner == consumer {
				err = e.dependOnPackage(consumer, resolution.plugin)
				break
			}
		}
	}
	if err == nil {
		err = e.validateRegistry()
	}
	return value, err
}
