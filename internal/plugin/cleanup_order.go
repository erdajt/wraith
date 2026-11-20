package plugin

import (
	"cmp"
	"slices"
)

// orderCleanupHooks keeps dependents alive through their own teardown, including removed packages.
func orderCleanupHooks(hooks []cleanupHook) []cleanupHook {
	hooks = slices.Clone(hooks)
	slices.SortStableFunc(hooks, func(a, b cleanupHook) int { return cmp.Compare(a.id, b.id) })

	grouped := map[string][]cleanupHook{}
	dependencies := map[string][]string{}
	for _, hook := range hooks {
		grouped[hook.owner] = append(grouped[hook.owner], hook)
		dependencies[hook.owner] = append(dependencies[hook.owner], hook.dependencies...)
	}
	var ordered []cleanupHook
	visited := map[string]bool{}
	var visit func(string)
	visit = func(owner string) {
		if visited[owner] {
			return
		}
		visited[owner] = true
		for _, dependency := range dependencies[owner] {
			visit(dependency)
		}
		ordered = append(ordered, grouped[owner]...)
	}
	for _, hook := range hooks {
		visit(hook.owner)
	}
	return ordered
}

// cleanupDependencies retains transitive edges even when intermediate packages have no hooks.
func (e *engine) cleanupDependencies(owner string) []string {
	graph := map[string][]string{}
	for _, record := range e.packages {
		graph[record.owner] = record.dependencies
	}
	var dependencies []string
	visited := map[string]bool{owner: true}
	var visit func(string)
	visit = func(current string) {
		for _, dependency := range graph[current] {
			if visited[dependency] {
				continue
			}
			visited[dependency] = true
			dependencies = append(dependencies, dependency)
			visit(dependency)
		}
	}
	visit(owner)
	return dependencies
}
