package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/plugin"
)

// regionFixture publishes a typed frame through the host's checked surface boundary.
func regionFixture(t *testing.T, a *App, owner string, region plugin.SurfaceRegion) plugin.SurfaceRegion {
	t.Helper()
	store := a.plugins.Surfaces()
	id, err := store.Reserve(owner, "region")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "region.create", ID: id, Owner: owner, Lease: store.OwnerLease(owner), Region: &region}); err != nil {
		t.Fatal(err)
	}
	return a.surfaceRegions[id]
}

func TestSurfaceRegionsReplaceRestoreAndReserveLayout(t *testing.T) {
	a := pluginFixture(t, "")
	a.width, a.height = 90, 28
	regionFixture(t, a, "core", plugin.SurfaceRegion{Slot: "header", Lines: []string{"base header"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "header", Lines: []string{"owned header"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "sidebar", Width: 24, Lines: []string{"scope", "local only"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "footer", Lines: []string{"owned footer"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "welcome", Width: 16, Lines: []string{"custom workspace"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "prompt", Lines: []string{"custom prompt"}})
	rows := strings.Split(ansi.Strip(a.View()), "\n")
	if !strings.HasPrefix(rows[0], "owned header") || !strings.Contains(rows[a.height-2], "owned footer") || !strings.Contains(strings.Join(rows, "\n"), "custom workspace") || !strings.Contains(rows[4+a.bodyHeight()], "custom prompt") {
		t.Fatal("replacement regions were not visible in their slots")
	}
	if a.mainWidth() != 90 || a.conversationWidth() != 65 || a.windowPlaces()[0].width != 65 || !strings.Contains(rows[3][65:], "scope") {
		t.Fatal("sidebar did not reserve only the conversation body")
	}
	a.commandMode, a.command = true, "settings"
	if rows = strings.Split(ansi.Strip(a.View()), "\n"); !strings.Contains(rows[a.height-2], ":settings") {
		t.Fatal("footer replacement hid active command entry")
	}
	a.plugins.Surfaces().RevokeOwner("config")
	a.reconcileSurfaces()
	if a.conversationWidth() != a.mainWidth() || !strings.HasPrefix(ansi.Strip(a.View()), "base header") {
		t.Fatal("unload did not restore previous header and layout width")
	}
}

func TestSurfaceBodyFramesClipAndOrderBelowDialogs(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("input",function()
 wraith.ui.input({title="modal title"}, function() end)
end)`)
	a.width, a.height = 50, 18
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "body", Width: 100, Height: 200, Column: 45, Row: 4, Lines: []string{"clipped body", "never below body"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "body", Width: 40, Height: 3, Lines: []string{"first layer"}})
	regionFixture(t, a, "config", plugin.SurfaceRegion{Slot: "body", Width: 40, Height: 3, Lines: []string{"second layer"}})
	rows := strings.Split(ansi.Strip(a.View()), "\n")
	if !strings.HasPrefix(rows[3], "second layer") || strings.Contains(strings.Join(rows[3+a.bodyHeight():], "\n"), "never below body") {
		t.Fatal("body region order or clipping changed native prompt rows")
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != a.width {
			t.Fatalf("frame escaped terminal width: %d", ansi.StringWidth(row))
		}
	}
	result := a.runCommand("input")().(pluginResultMsg)
	a.openPluginDialog(a.plugins, result.Result.Effects[0].Data.(plugin.Dialog))
	if !strings.Contains(ansi.Strip(a.View()), "modal title") {
		t.Fatal("passive body frame obscured an active dialog")
	}
}
