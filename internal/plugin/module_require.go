package plugin

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

type moduleRequest struct {
	name       string
	resolution *moduleResolution
	owner      string
}

// requireModule routes managed imports while preserving native fallback stack and errors.
func (e *engine) requireModule(L *lua.LState, native nativeModules) int {
	name := L.CheckString(1)
	state, consumer, calls := e.L, e.owner, len(e.packageCalls)
	e.L = L
	e.packageCalls = append(e.packageCalls, consumer)
	defer func() {
		e.L, e.owner = state, consumer
		e.packageCalls = e.packageCalls[:calls]
	}()

	if e.closing {
		value, err := e.closingModuleValue(L, name, native.pkg, native.loaded, native.preload)
		if err != nil {
			L.RaiseError("%v", err)
		}
		L.Push(value)
		return 1
	}

	metadata := len(e.moduleResolving) > 0
	pure := e.pureHook && (len(e.moduleProviders) > 0 || len(e.moduleClaims) > 0)
	if metadata || pure {
		err := native.validate(L)
		value := native.loaded.RawGetString(name)
		if err == nil && (e.moduleClaims[name] != nil || !lua.LVAsBool(value) || L.IsModuleLoading(name)) {
			err = fmt.Errorf("require of %s is unavailable during module metadata or pure hooks", name)
		}
		if err != nil {
			if metadata {
				e.moduleViolation = err
			}
			if pure {
				e.hookViolation = err
			}
			L.RaiseError("%v", err)
		}
		L.Push(value)
		return 1
	}

	if len(name) > 128 || !namespaceName.MatchString(name) {
		return native.require.GFunction(L)
	}
	if claim := e.moduleClaims[name]; claim != nil && claim.poison != "" {
		L.RaiseError("module %s is poisoned; use /reload: %s", name, claim.poison)
	}

	var request *moduleRequest
	for index := len(e.moduleRequests) - 1; index >= 0; index-- {
		if e.moduleRequests[index].name == name {
			request = e.moduleRequests[index]
			break
		}
	}
	if request == nil {
		if len(e.moduleProviders) == 0 && e.moduleClaims[name] == nil {
			return native.require.GFunction(L)
		}
		resolution, err := e.resolveModuleProvider(L, name)
		if err != nil {
			L.RaiseError("%v", err)
		}
		if resolution == nil {
			if e.moduleClaims[name] != nil {
				L.RaiseError("module %s has no resolver; restore its provider or use /reload", name)
			}
			return native.require.GFunction(L)
		}
		request = &moduleRequest{name: name, resolution: resolution}
	}

	value, err := e.runModuleRequest(L, request, consumer, native)
	if err != nil {
		L.RaiseError("%v", err)
	}
	L.Push(value)
	return 1
}

// runModuleRequest shares selected generations while each nested caller retains its own dependency.
func (e *engine) runModuleRequest(L *lua.LState, request *moduleRequest, consumer string, native nativeModules) (value lua.LValue, err error) {
	if len(e.moduleRequests) >= 32 {
		return lua.LNil, errors.New("managed module request nesting exceeds 32")
	}
	if err := native.validate(L); err != nil {
		return lua.LNil, err
	}
	release, err := e.claimModule(request.name)
	if err != nil {
		return lua.LNil, err
	}
	defer release()

	depth, calls, top := len(e.moduleRequests), len(e.packageCalls), L.GetTop()
	e.moduleRequests = append(e.moduleRequests, request)
	e.packageCalls = append(e.packageCalls, request.resolution.provider.owner)
	before := e.captureRegistry()
	initial := native.loaded.RawGetString(request.name)
	defer func() {
		if err != nil {
			value = lua.LNil
			err = e.rollbackModule(L, request.name, native, initial, before, err)
		}
		L.SetTop(top)
		e.moduleRequests = e.moduleRequests[:depth]
		e.packageCalls = e.packageCalls[:calls]
	}()

	owner := ""
	if L.IsModuleLoading(request.name) {
		owner = e.packages[request.resolution.plugin].owner
	} else {
		owner, err = e.activateModuleProvider(L, request.name, request.resolution, consumer, native)
		if err != nil {
			return lua.LNil, err
		}
	}
	if request.owner != "" && request.owner != owner {
		return lua.LNil, errors.New("module provider generation changed during activation")
	}
	request.owner = owner
	return e.invokeOwnedModule(L, request.name, request.resolution, consumer, owner, native)
}
