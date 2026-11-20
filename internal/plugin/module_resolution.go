package plugin

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type moduleResolution struct {
	provider        moduleProvider
	plugin          string
	activate, check *lua.LFunction
}

type moduleVerification struct {
	phase    string
	proof    lua.LValue
	executed bool
}

// decodeModuleResolution detaches a strict continuation from mutable resolver output.
func decodeModuleResolution(L *lua.LState, value lua.LValue, provider moduleProvider) (*moduleResolution, error) {
	if value == lua.LNil {
		return nil, nil
	}
	table, ok := value.(*lua.LTable)
	if !ok || L.GetMetatable(table) != lua.LNil {
		return nil, errors.New("module resolver requires a plain descriptor table or nil")
	}
	for key, _ := table.Next(lua.LNil); key != lua.LNil; key, _ = table.Next(key) {
		if key != lua.LString("plugin") && key != lua.LString("activate") && key != lua.LString("check") {
			return nil, errors.New("module descriptor contains an unknown field")
		}
	}
	plugin, named := table.RawGetString("plugin").(lua.LString)
	activate, activates := table.RawGetString("activate").(*lua.LFunction)
	check, checks := table.RawGetString("check").(*lua.LFunction)
	if !named || len(plugin) > 128 || !namespaceName.MatchString(string(plugin)) || !activates || !checks {
		return nil, errors.New("module descriptor requires plugin, activate and check")
	}
	return &moduleResolution{provider: provider, plugin: string(plugin), activate: activate, check: check}, nil
}

// guardModuleMetadata rejects host effects even when a resolver catches the lua error.
func (e *engine) guardModuleMetadata(L *lua.LState, operation string) {
	if len(e.moduleResolving) == 0 {
		return
	}
	allowed := strings.HasPrefix(operation, "wraith.json.") || slices.Contains([]string{
		"wraith.modules.source", "wraith.modules.catalog", "wraith.modules.loading", "wraith.fs.stat", "wraith.plugins.list", "wraith.plugins.state",
	}, operation)
	if operation == "wraith.fs.stat" {
		if options, ok := L.Get(2).(*lua.LTable); ok && options.RawGetString("async") != lua.LNil || L.Get(2).Type() == lua.LTFunction {
			allowed = false
		}
	}
	if !allowed {
		e.moduleViolation = fmt.Errorf("%s is unavailable during module metadata resolution", operation)
		L.RaiseError("%v", e.moduleViolation)
	}
}

// withModuleMetadata bounds callbacks while retaining caught violations and the caller state.
func (e *engine) withModuleMetadata(L *lua.LState, name string, call func() error) (err error) {
	depth := len(e.moduleResolving)
	if len(name) > 128 || !namespaceName.MatchString(name) || depth >= 32 || slices.Contains(e.moduleResolving, name) {
		err = fmt.Errorf("invalid or recursive module resolution: %s", name)
		if depth > 0 {
			e.moduleViolation = err
		}
		return err
	}
	state := e.L
	e.L = L
	defer func() { e.L = state }()

	before := e.captureRegistry()
	owner, calls, top := e.owner, len(e.packageCalls), L.GetTop()
	previous := e.moduleViolation
	if depth == 0 {
		e.moduleViolation = nil
	}
	e.moduleResolving = append(e.moduleResolving, name)
	e.packageCalls = append(e.packageCalls, owner)
	defer func() {
		if err == nil {
			err = e.moduleViolation
		} else if e.moduleViolation != nil && err != e.moduleViolation {
			err = errors.Join(err, e.moduleViolation)
		}
		e.moduleResolving = e.moduleResolving[:depth]
		e.packageCalls, e.owner = e.packageCalls[:calls], owner
		L.SetTop(top)
		if depth == 0 {
			e.moduleViolation = previous
		} else if e.moduleViolation == nil && err != nil {
			e.moduleViolation = err
		}
		if err != nil {
			err = errors.Join(err, e.disposeRegistry(e.restoreRegistry(before)))
		}
	}()
	return call()
}

// resolveModuleProvider evaluates ordered metadata without activating plugins or publishing effects.
func (e *engine) resolveModuleProvider(L *lua.LState, name string) (result *moduleResolution, err error) {
	err = e.withModuleMetadata(L, name, func() error {
		calls := len(e.packageCalls)
		for _, provider := range slices.Clone(e.moduleProviders) {
			if !slices.Contains(e.moduleProviders, provider) {
				return errors.New("module resolver registration changed")
			}
			e.owner = provider.owner
			e.packageCalls = append(e.packageCalls[:calls], provider.owner)
			if err := L.CallByParam(lua.P{Fn: provider.resolve, NRet: 1, Protect: true}, lua.LString(name)); err != nil {
				return err
			}
			if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			if e.moduleViolation != nil {
				return e.moduleViolation
			}
			if !slices.Contains(e.moduleProviders, provider) {
				return errors.New("module resolver registration changed")
			}
			result, err = decodeModuleResolution(L, L.Get(-1), provider)
			L.Pop(1)
			if err != nil || result != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// checkModuleResolution validates a captured continuation under its original resolver owner.
func (e *engine) checkModuleResolution(L *lua.LState, name string, resolution *moduleResolution, verification moduleVerification) (lua.LValue, error) {
	if !slices.Contains([]string{"validate", "before", "after"}, verification.phase) {
		return lua.LNil, errors.New("invalid module verification phase")
	}
	if verification.proof == nil {
		verification.proof = lua.LNil
	}
	value := lua.LValue(lua.LNil)
	err := e.withModuleMetadata(L, name, func() error {
		if resolution == nil || resolution.check == nil || !slices.Contains(e.moduleProviders, resolution.provider) {
			return errors.New("module resolver registration changed")
		}
		e.owner = resolution.provider.owner
		e.packageCalls = append(e.packageCalls, resolution.provider.owner)
		if err := L.CallByParam(lua.P{Fn: resolution.check, NRet: 1, Protect: true},
			lua.LString(name), lua.LString(verification.phase), verification.proof, lua.LBool(verification.executed)); err != nil {
			return err
		}
		if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if !slices.Contains(e.moduleProviders, resolution.provider) {
			return errors.New("module resolver registration changed")
		}
		value = L.Get(-1)
		return nil
	})
	if err != nil {
		return lua.LNil, err
	}
	return value, nil
}
