package plugin

import (
	"context"
	"reflect"
	"slices"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestCleanupRetainsTransitiveDependenciesAndRestoresOwner(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	L.SetContext(context.Background())
	e := &engine{L: L, packages: map[string]packageRecord{
		"parent": {owner: "parent", dependencies: []string{"middle"}},
		"middle": {owner: "middle", dependencies: []string{"child"}},
		"child":  {owner: "child"},
	}}
	e.owner = "caller"
	var owners []string
	cleanup := L.NewFunction(func(*lua.LState) int { owners = append(owners, e.owner); return 0 })
	hooks := []cleanupHook{
		{owner: "parent", function: cleanup, dependencies: e.cleanupDependencies("parent")},
		{owner: "child", function: cleanup},
	}
	e.packages = nil
	if err := e.disposeHooks(hooks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(owners, []string{"parent", "child"}) || e.owner != "caller" {
		t.Fatalf("cleanup order or owner changed: %v, current %q", owners, e.owner)
	}
}

// TestCleanupUsesRegistrationOrderAcrossStagedGroups retains hook order independently of unload order.
func TestCleanupUsesRegistrationOrderAcrossStagedGroups(t *testing.T) {
	for _, dependencies := range []bool{false, true} {
		for _, staged := range [][]int{{0, 1, 2, 3}, {2, 3, 0, 1}, {3, 1, 2, 0}} {
			L := lua.NewState()
			L.SetContext(t.Context())
			e := &engine{L: L, owner: "caller"}
			var calls []int
			registered := make([]cleanupHook, 4)
			for index := range registered {
				owner := "early"
				if index >= 2 {
					owner = "late"
				}
				registered[index] = cleanupHook{id: uint64(index + 1), owner: owner,
					function: L.NewFunction(func(*lua.LState) int { calls = append(calls, index); return 0 })}
				if dependencies && owner == "early" {
					registered[index].dependencies = []string{"late"}
				}
			}
			hooks := make([]cleanupHook, 0, len(staged))
			for _, index := range staged {
				hooks = append(hooks, registered[index])
			}
			before := slices.Clone(hooks)
			err := e.disposeHooks(hooks)
			L.Close()
			if err != nil {
				t.Fatal(err)
			}
			want := []int{3, 2, 1, 0}
			if dependencies {
				want = []int{1, 0, 3, 2}
			}
			if !slices.Equal(calls, want) || !reflect.DeepEqual(hooks, before) || e.owner != "caller" {
				t.Fatalf("staged %v, dependency %t: cleanup %v, want %v; caller or input changed", staged, dependencies, calls, want)
			}
		}
	}
}
