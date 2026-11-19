package plugin

import (
	"errors"

	lua "github.com/yuin/gopher-lua"
)

type moduleExecution struct {
	name, owner string
	cache       *lua.LTable
	value       lua.LValue
	serial      uint64
}

type registryDisposal struct {
	hooks   []cleanupHook
	modules []moduleExecution
}

// recordModuleExecution retains the latest verified result without growing on repeated loads.
func (e *engine) recordModuleExecution(name, owner string, cache *lua.LTable) error {
	if e.closing || owner == "" || cache == nil || len(name) > 128 || !namespaceName.MatchString(name) {
		return errors.New("invalid module execution receipt")
	}
	owned := false
	for _, record := range e.packages {
		owned = owned || record.owner == owner
	}
	if !owned {
		return errors.New("module execution requires a live plugin owner")
	}
	if _, exists := e.moduleExecutions[name]; !exists && len(e.moduleExecutions) >= 1024 {
		return errors.New("module execution receipts exceed 1024")
	}
	if e.moduleExecutions == nil {
		e.moduleExecutions = map[string]moduleExecution{}
	}
	e.serial++
	e.moduleExecutions[name] = moduleExecution{name: name, owner: owner, cache: cache, value: cache.RawGetString(name), serial: e.serial}
	return nil
}

// takeModuleExecutions detaches invalidation work from the live execution ledger.
func (e *engine) takeModuleExecutions(match func(moduleExecution) bool) []moduleExecution {
	var selected []moduleExecution
	for name, execution := range e.moduleExecutions {
		if match(execution) {
			selected = append(selected, execution)
			delete(e.moduleExecutions, name)
		}
	}
	return selected
}

// retiredModuleExecutions selects old generations only after their removal is accepted.
func (e *engine) retiredModuleExecutions() []moduleExecution {
	owners := make(map[string]bool, len(e.packages))
	for _, record := range e.packages {
		owners[record.owner] = true
	}
	return e.takeModuleExecutions(func(execution moduleExecution) bool { return !owners[execution.owner] })
}

// disposeRegistry lets cleanup read cached exports before invalidating only proven values.
func (e *engine) disposeRegistry(disposal registryDisposal) error {
	err := e.disposeHooks(disposal.hooks)
	for _, execution := range disposal.modules {
		if execution.cache.RawGetString(execution.name) == execution.value {
			execution.cache.RawSetString(execution.name, lua.LNil)
		}
	}
	return err
}
