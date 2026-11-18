package plugin

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestDeleteOverridesAnotherOwnerUntilTheDeletingOwnerLeaves(t *testing.T) {
	e, err := load(t.Context(), t.TempDir(), config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.L.Close)
	e.L.SetContext(t.Context())
	e.owner = "extension"
	if err := e.L.DoString(`
wraith.commands.create("inspect", function() wraith.notify("extension") end)
wraith.keymap.set("prompt.normal", "<C-m>", function() end)`); err != nil {
		t.Fatal(err)
	}
	e.owner = "config"
	if err := e.L.DoString(`
wraith.commands.delete("inspect")
wraith.keymap.del("prompt.normal", "<enter>")`); err != nil {
		t.Fatal(err)
	}
	e.refreshManifest()
	if len(e.manifest.Commands) != 0 || len(e.manifest.Config.Keymaps) != 0 || e.actions["command.inspect"] {
		t.Fatal("configuration could not hide extension registrations")
	}
	if len(e.functions) != 2 {
		t.Fatal("hidden extension callbacks were released before owner removal")
	}
	e.removeOwnerRegistrations("config")
	if len(e.manifest.Commands) != 1 || len(e.manifest.Config.Keymaps) != 1 || !e.actions["command.inspect"] {
		t.Fatal("removing deletion overrides did not restore the extension")
	}
	e.removeOwnerRegistrations("extension")
	e.refreshManifest()
	if len(e.functions) != 0 {
		t.Fatal("removed owners retained callbacks")
	}
}

func TestFailedConfigurationChangePreservesOwnedBindings(t *testing.T) {
	r := openTestRuntime(t, `
wraith.keymap.set("prompt.normal", "x", function() wraith.notify("original") end)
wraith.commands.create("change", function()
  wraith.config.setup({keymaps = {}})
  error("discard configuration")
end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveResult(t, pending); got.Err == nil {
		t.Fatal("failed configuration callback succeeded")
	}
	pending, err = r.Submit(m.Config.Keymaps[0].Action, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || got.Manifest != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "original" {
		t.Fatalf("rollback did not retain the original mapping: %+v", got)
	}
}

func TestEventListenerLimitDoesNotRetainRejectedCallbacks(t *testing.T) {
	r := openTestRuntime(t, `
for i = 1, 32 do wraith.events.on("project.changed", function() end) end
for i = 1, 600 do
  assert(not pcall(wraith.events.on, "project.changed", function() end))
end`)
	if got := len(r.Manifest().Callbacks); got != 32 {
		t.Fatalf("rejected listeners retained callbacks: got %d want 32", got)
	}
}
