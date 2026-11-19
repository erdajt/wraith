package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

// TestStartupSurfacesPreserveProceduralPlacement exercises creation order beyond lexical handle sorting.
func TestStartupSurfacesPreserveProceduralPlacement(t *testing.T) {
	a := pluginFixture(t, `
local buffer = wraith.buffers.create({lines={"shared"}, editable=true})
local previous
for i=1,12 do
 previous = wraith.windows.open(buffer, {position="right", relative_to=previous, focus=true})
end
wraith.commands.create("inspect", function()
 assert(#wraith.buffers.list() >= 3)
 wraith.notify("startup ready")
end)`)
	deltas, err := preparePluginSurfaces(a.plugins, 1)
	if err != nil {
		t.Fatal(err)
	}
	a.installPluginSurfaces(deltas)
	if len(a.windows) != 13 || a.focusedSurfaceView() == nil {
		t.Fatal("startup views or focus missing")
	}
	places := a.windowTree.layout(windowRect{width: 200000, height: 40}, a.windowID)
	previous := 0
	for _, delta := range deltas {
		if delta.Window == nil {
			continue
		}
		id := a.surfaceWindows[delta.Window.ID]
		found := false
		for _, place := range places {
			if place.id == id {
				if place.x < previous {
					t.Fatal("startup placements were reordered")
				}
				previous = place.x
				found = true
			}
		}
		if !found {
			t.Fatal("startup view missing from split tree")
		}
	}
	a.Update(a.runCommand("inspect")())
	if a.notice != "startup ready" {
		t.Fatal(a.notice)
	}
}

// TestRejectedSurfaceReloadPreservesCurrentWorkspace checks host-only capacity before installation.
func TestRejectedSurfaceReloadPreservesCurrentWorkspace(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("alive",function() wraith.notify("still active") end)`)
	a.width, a.height, a.focus = 120, 40, "chat"
	a.runAction("window.split_vertical", "")
	old := a.plugins
	if err := config.Save(config.File(a.Paths.Config), a.Config); err != nil {
		t.Fatal(err)
	}
	source := `local b=wraith.buffers.create({lines={"candidate"}})
for i=1,63 do wraith.windows.open(b,{}) end`
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	a.Update(a.reloadPlugins()())
	if a.plugins != old || len(a.windows) != 2 || !strings.Contains(a.notice, "window limit") {
		t.Fatalf("rejected reload changed workspace: %s", a.notice)
	}
	a.Update(a.runCommand("alive")())
	if a.notice != "still active" {
		t.Fatal(a.notice)
	}
}
