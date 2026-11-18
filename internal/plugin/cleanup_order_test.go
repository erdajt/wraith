package plugin

import (
	"context"
	"reflect"
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
