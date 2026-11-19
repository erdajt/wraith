package plugin

import (
	"errors"
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type moduleClaim struct {
	pending int
	poison  string
}

// claimModule reserves lifetime capacity before nested activation can expose a source.
func (e *engine) claimModule(name string) (func(), error) {
	if len(name) > 128 || !namespaceName.MatchString(name) {
		return nil, errors.New("invalid managed module name")
	}
	claim := e.moduleClaims[name]
	if claim == nil {
		if len(e.moduleClaims) >= 1024 {
			return nil, errors.New("managed module claims exceed 1024; use /reload")
		}
		if e.moduleClaims == nil {
			e.moduleClaims = map[string]*moduleClaim{}
		}
		claim = &moduleClaim{}
		e.moduleClaims[name] = claim
	}
	if claim.poison != "" {
		return nil, fmt.Errorf("module %s is poisoned; use /reload: %s", name, claim.poison)
	}
	if claim.pending >= 64 {
		return nil, errors.New("module claim nesting exceeds 64")
	}
	claim.pending++
	released := false
	return func() {
		if !released {
			claim.pending--
			released = true
		}
	}, nil
}

// poisonModule retains the first failed proof without discarding foreign cache values.
func (e *engine) poisonModule(name, reason string) error {
	claim := e.moduleClaims[name]
	if claim == nil {
		return errors.New("cannot poison an unclaimed module")
	}
	if claim.poison == "" {
		if reason == "" {
			reason = "module ownership could not be verified"
		}
		claim.poison = strings.ToValidUTF8(reason[:min(len(reason), 1024)], "?")
	}
	return nil
}

// closingModuleValue reads verified exports without resolving, activating or executing sources.
func (e *engine) closingModuleValue(L *lua.LState, name string, pkg, loaded, preload *lua.LTable) (lua.LValue, error) {
	if !e.closing || pkg == nil || loaded == nil || preload == nil || L.GetGlobal("package") != pkg ||
		pkg.RawGetString("loaded") != loaded || pkg.RawGetString("preload") != preload ||
		L.Get(lua.RegistryIndex).(*lua.LTable).RawGetString("_LOADED") != loaded || L.GetMetatable(loaded) != lua.LNil {
		return lua.LNil, errors.New("module tables changed during cleanup; use /reload")
	}
	value := loaded.RawGetString(name)
	if !lua.LVAsBool(value) || L.IsModuleLoading(name) {
		return lua.LNil, fmt.Errorf("module %s requires source execution during cleanup", name)
	}
	claim := e.moduleClaims[name]
	if claim != nil && claim.poison != "" {
		return lua.LNil, fmt.Errorf("module %s is poisoned; use /reload: %s", name, claim.poison)
	}
	newest, found := e.moduleExecutions[name]
	for _, frame := range e.moduleDisposals {
		for _, execution := range frame {
			if execution.name == name && (!found || execution.serial > newest.serial) {
				newest, found = execution, true
			}
		}
	}
	if claim == nil && !found {
		return value, nil
	}
	if !found || newest.cache != loaded || newest.value != value {
		return lua.LNil, fmt.Errorf("module %s has no matching cleanup receipt; use /reload", name)
	}
	return value, nil
}
