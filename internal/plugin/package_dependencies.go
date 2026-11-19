package plugin

import (
	"errors"
	"fmt"
	"slices"
)

// dependOnPackage records an existing dependency without loading it under another owner.
func (e *engine) dependOnPackage(owner, name string) error {
	target, ok := e.packages[name]
	if !ok || target.loading {
		return fmt.Errorf("dependency is not fully loaded: %s", name)
	}
	caller := ""
	for name, record := range e.packages {
		if record.owner == owner {
			caller = name
			break
		}
	}
	if caller == "" {
		return errors.New("dependency declarations require a plugin owner")
	}
	if target.owner == owner {
		return fmt.Errorf("plugin cannot depend on itself: %s", name)
	}

	pending, seen := []string{target.owner}, map[string]bool{}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == owner {
			return fmt.Errorf("plugin dependency cycle: %s -> %s", caller, name)
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		for _, record := range e.packages {
			if record.owner == current {
				pending = append(pending, record.dependencies...)
			}
		}
	}
	record := e.packages[caller]
	if slices.Contains(record.dependencies, target.owner) {
		return nil
	}
	record.dependencies = append(record.dependencies, target.owner)
	e.packages[caller] = record
	for i := range e.cleanup {
		e.cleanup[i].dependencies = e.cleanupDependencies(e.cleanup[i].owner)
	}
	e.dirty = !e.setup
	return nil
}
