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
