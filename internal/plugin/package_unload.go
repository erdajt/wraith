package plugin

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// unloadPlugins stages a complete cohort in dependent-first order without running teardown.
func (e *engine) unloadPlugins(names []string) error {
	if e.closing {
		return errors.New("plugin unload is disabled during teardown")
	}
	if len(names) > 128 {
		return errors.New("plugins.unload_many accepts at most 128 names")
	}
	if len(names) == 0 {
		return nil
	}
	names = slices.Clone(names)
	slices.Sort(names)
	targets, owners := map[string]bool{}, map[string]string{}
	for _, name := range names {
		if targets[name] {
			return fmt.Errorf("duplicate plugin unload target: %s", name)
		}
		record, ok := e.packages[name]
		if !ok {
			return fmt.Errorf("plugin is not loaded: %s", name)
		}
		if record.loading || record.owner == e.owner || slices.Contains(e.packageCalls, record.owner) {
			return fmt.Errorf("cannot unload the currently executing plugin: %s", name)
		}
		if _, duplicate := owners[record.owner]; record.owner == "" || duplicate {
			return errors.New("plugin unload found an invalid owner registry")
		}
		targets[name], owners[record.owner] = true, name
	}
	for _, name := range slices.Sorted(maps.Keys(e.packages)) {
		if targets[name] {
			continue
		}
		for _, owner := range e.packages[name].dependencies {
			if target, ok := owners[owner]; ok {
				return fmt.Errorf("plugin %s is required by %s", target, name)
			}
		}
	}

	dependents, dependencies := map[string]int{}, map[string][]string{}
	for _, name := range names {
		for _, owner := range e.packages[name].dependencies {
			if target, ok := owners[owner]; ok && !slices.Contains(dependencies[name], target) {
				dependencies[name] = append(dependencies[name], target)
				dependents[target]++
			}
		}
	}
	order := make([]string, 0, len(names))
	for len(order) < len(names) {
		ready, found := "", false
		for _, name := range names {
			if targets[name] && dependents[name] == 0 {
				ready, found = name, true
				break
			}
		}
		if !found {
			return errors.New("plugin dependency cycle prevents grouped unload")
		}
		order = append(order, ready)
		delete(targets, ready)
		for _, dependency := range dependencies[ready] {
			dependents[dependency]--
		}
	}

	before := e.captureRegistry()
	for _, name := range order {
		if err := e.unloadPlugin(name); err != nil {
			return errors.Join(err, e.disposeRegistry(e.restoreRegistry(before)))
		}
	}
	return nil
}
