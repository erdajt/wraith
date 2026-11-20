package plugin

import (
	"errors"
	"fmt"
	"slices"

	lua "github.com/yuin/gopher-lua"
)

type modulePreparation struct {
	provider moduleProvider
	plugin   string
}

// retiringModuleProvider detects exposed generations awaiting publication acceptance.
func (e *engine) retiringModuleProvider(plugin, owner string) bool {
	for _, execution := range e.moduleExecutions {
		if execution.plugin == plugin && execution.owner != owner {
			return true
		}
	}
	return false
}

// validateModuleActivation checks provenance against the current cache and lifecycle snapshot.
func (e *engine) validateModuleActivation(L *lua.LState, name string, resolution *moduleResolution, native nativeModules) (packageRecord, bool, error) {
	if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
		return packageRecord{}, false, ctx.Err()
	}
	if err := native.validate(L); err != nil {
		return packageRecord{}, false, err
	}
	if !slices.Contains(e.moduleProviders, resolution.provider) {
		return packageRecord{}, false, errors.New("module resolver registration changed")
	}
	record, present := e.packages[resolution.plugin]
	if e.retiringModuleProvider(resolution.plugin, record.owner) {
		return packageRecord{}, false, fmt.Errorf("module provider %s is awaiting retirement", resolution.plugin)
	}
	initial := native.loaded.RawGetString(name)
	_, err := e.checkModuleResolution(L, name, resolution, moduleVerification{phase: "validate"})
	if native.validate(L) != nil || native.loaded.RawGetString(name) != initial {
		err = errors.Join(err, errors.New("module validation changed native loading state; use /reload"))
		err = errors.Join(err, e.poisonModule(name, err.Error()))
	}
	if err != nil {
		return packageRecord{}, false, err
	}
	current, exists := e.packages[resolution.plugin]
	if present != exists || current.owner != record.owner || current.loading != record.loading {
		return packageRecord{}, false, errors.New("module provider changed during validation")
	}
	return record, present, nil
}

// activateModuleProvider prepares a lifecycle without invoking its requested source.
func (e *engine) activateModuleProvider(L *lua.LState, name string, resolution *moduleResolution, consumer string, native nativeModules) (owner string, err error) {
	if len(e.moduleResolving) > 0 {
		e.moduleViolation = errors.New("module activation is unavailable during metadata resolution")
		return "", e.moduleViolation
	}
	if e.closing || resolution == nil || resolution.activate == nil || resolution.check == nil || !slices.Contains(e.moduleProviders, resolution.provider) {
		return "", errors.New("module activation requires a current resolver outside cleanup")
	}
	if err := native.validate(L); err != nil {
		return "", err
	}
	if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
		return "", ctx.Err()
	}
	release, err := e.claimModule(name)
	if err != nil {
		return "", err
	}
	defer release()
	state, previous, calls, top := e.L, e.owner, len(e.packageCalls), L.GetTop()
	e.L = L
	e.packageCalls = append(e.packageCalls, consumer, resolution.provider.owner)
	before := e.captureRegistry()
	depth, preparationDepth := len(e.moduleActivating), len(e.modulePreparing)
	initial := native.loaded.RawGetString(name)
	defer func() {
		if err != nil {
			owner = ""
			err = e.rollbackModule(L, name, native, initial, before, err)
		}
		L.SetTop(top)
		e.L, e.owner = state, previous
		e.packageCalls = e.packageCalls[:calls]
		e.moduleActivating = e.moduleActivating[:depth]
		e.modulePreparing = e.modulePreparing[:preparationDepth]
	}()
	record, present, err := e.validateModuleActivation(L, name, resolution, native)
	if err != nil {
		return "", err
	}
	if !present {
		if depth+preparationDepth >= 32 || slices.Contains(e.moduleActivating, resolution.plugin) {
			return "", fmt.Errorf("recursive module activation or depth exceeds 32: %s", resolution.plugin)
		}
		preparation := modulePreparation{provider: resolution.provider, plugin: resolution.plugin}
		if resolution.prepare != nil && !slices.Contains(e.modulePreparing, preparation) {
			e.modulePreparing = append(e.modulePreparing, preparation)
			e.owner = resolution.provider.owner
			if err = L.CallByParam(lua.P{Fn: resolution.prepare, NRet: 0, Protect: true}, lua.LString(name)); err != nil {
				return "", err
			}
			e.modulePreparing = e.modulePreparing[:preparationDepth]
			record, present, err = e.validateModuleActivation(L, name, resolution, native)
			if err != nil {
				return "", err
			}
		}
	}
	if present {
		if record.owner == "" {
			return "", errors.New("module provider has no lifecycle owner")
		}
		if record.loading && consumer != record.owner {
			return "", fmt.Errorf("module provider is still loading: %s", resolution.plugin)
		}
		if err = e.validateRegistry(); err != nil {
			return "", err
		}
		return record.owner, nil
	}
	e.moduleActivating = append(e.moduleActivating, resolution.plugin)
	e.owner = resolution.provider.owner
	if err = L.CallByParam(lua.P{Fn: resolution.activate, NRet: 0, Protect: true}, lua.LString(name)); err != nil {
		return "", err
	}
	if ctx := L.Context(); ctx != nil && ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err = native.validate(L); err != nil {
		err = errors.Join(err, e.poisonModule(name, err.Error()))
		return "", err
	}
	record, present = e.packages[resolution.plugin]
	if !present || record.loading || record.owner == "" || !slices.Contains(e.moduleProviders, resolution.provider) {
		return "", errors.New("module activation did not leave a ready provider and current resolver")
	}
	if e.retiringModuleProvider(resolution.plugin, record.owner) {
		return "", fmt.Errorf("module provider %s replaced a retiring generation", resolution.plugin)
	}
	if err = e.validateRegistry(); err != nil {
		return "", err
	}
	return record.owner, nil
}
