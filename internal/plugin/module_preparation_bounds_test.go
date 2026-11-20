package plugin

import (
	"fmt"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestModulePreparationSeparatesResolverRegistrations keeps preparation frames tied to exact registrations.
func TestModulePreparationSeparatesResolverRegistrations(t *testing.T) {
	e, native := moduleRequireFixture(t)
	first := e.moduleProviders[0]
	e.owner = first.owner
	if _, err := e.addModuleProvider(first.resolve); err != nil {
		t.Fatal(err)
	}
	e.owner = "config"
	if err := e.L.DoString(`package.preload.provider=function() return {setup=function() end} end
function activate() wraith.plugins.load("provider", {}, {dependency=false}) end`); err != nil {
		t.Fatal(err)
	}

	preparations := [2]int{}
	resolutions := make([]*moduleResolution, 2)
	for index := range resolutions {
		resolutions[index] = &moduleResolution{
			provider: e.moduleProviders[index], plugin: "provider",
			activate: e.L.GetGlobal("activate").(*lua.LFunction),
			check:    e.L.NewFunction(func(*lua.LState) int { return 0 }),
			prepare: e.L.NewFunction(func(L *lua.LState) int {
				preparations[index]++
				if index == 0 {
					if _, err := e.activateModuleProvider(L, "nested", resolutions[1], "config", native); err != nil {
						L.RaiseError("%v", err)
					}
				}
				return 0
			}),
		}
	}
	if _, err := e.activateModuleProvider(e.L, "target", resolutions[0], "config", native); err != nil {
		t.Fatal(err)
	}
	if preparations != [2]int{1, 1} || len(e.packages) != 1 || len(e.modulePreparing) != 0 {
		t.Fatalf("different registrations shared a preparation frame: %v", preparations)
	}
}

// TestModulePreparationBoundsCombinedDepth exercises real alternating preparation and activation callbacks.
func TestModulePreparationBoundsCombinedDepth(t *testing.T) {
	for _, count := range []int{32, 33} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			e, native := moduleRequireFixture(t)
			resolutions := make([]*moduleResolution, count)
			visited := 0
			for index := range resolutions {
				name := fmt.Sprintf("provider%d", index)
				if err := e.L.DoString(fmt.Sprintf(`package.preload[%q]=function() return {setup=function() end} end`, name)); err != nil {
					t.Fatal(err)
				}
				load, err := e.L.LoadString(fmt.Sprintf(`wraith.plugins.load(%q, {}, {dependency=false})`, name))
				if err != nil {
					t.Fatal(err)
				}
				next := e.L.NewFunction(func(L *lua.LState) int {
					visited++
					if index+1 < count {
						if _, err := e.activateModuleProvider(L, fmt.Sprintf("source%d", index+1), resolutions[index+1], "config", native); err != nil {
							L.RaiseError("%v", err)
						}
					}
					if index%2 == 1 {
						L.CallByParam(lua.P{Fn: load, NRet: 0})
					}
					return 0
				})
				resolution := &moduleResolution{provider: e.moduleProviders[0], plugin: name, activate: load, check: e.L.NewFunction(func(*lua.LState) int { return 0 })}
				if index%2 == 0 {
					resolution.prepare = next
				} else {
					resolution.activate = next
				}
				resolutions[index] = resolution
			}

			_, err := e.activateModuleProvider(e.L, "source0", resolutions[0], "config", native)
			if count == 32 && (err != nil || len(e.packages) != 32) {
				t.Fatalf("valid combined depth failed: %v", err)
			}
			if count == 33 && (err == nil || !strings.Contains(err.Error(), "depth exceeds 32") || len(e.packages) != 0) {
				t.Fatalf("overflow escaped or retained lifecycles: %v", err)
			}
			if visited != 32 || len(e.modulePreparing) != 0 || len(e.moduleActivating) != 0 || e.owner != "config" {
				t.Fatalf("depth guard ran extra callbacks or leaked frames: %d", visited)
			}
		})
	}
}
