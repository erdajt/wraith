package plugin

import (
	"slices"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/keymap"
)

func ownershipFixture() *engine {
	e := &engine{manifest: Manifest{Config: config.Default()}, actions: map[string]bool{}, builtins: map[string]bool{}, events: map[string][]string{}}
	e.initOwnership()
	return e
}

func TestOwnerRemovalRestoresCommandsAndSubscriptions(t *testing.T) {
	e := ownershipFixture()
	for _, owner := range []string{"config", "first", "second"} {
		e.owner = owner
		e.setOwnedCommand(Command{Name: "inspect", Callback: owner})
		e.callbackOwners[owner] = owner
		e.events["project.changed"] = append(e.events["project.changed"], owner)
	}
	e.removeOwnerRegistrations("first")
	if e.manifest.Commands[0].Callback != "second" {
		t.Fatal("removing a covered owner changed the active command")
	}
	e.removeOwnerRegistrations("second")
	if e.manifest.Commands[0].Callback != "config" || !slices.Equal(e.events["project.changed"], []string{"config"}) {
		t.Fatalf("configuration was not restored: commands=%v events=%v", e.manifest.Commands, e.events)
	}
	e.removeOwnerRegistrations("config")
	if len(e.manifest.Commands) != 0 || e.actions["command.inspect"] {
		t.Fatal("removed custom command remained callable")
	}

	e.builtins["command.quit"] = true
	e.setOwnedCommand(Command{Name: "quit", Callback: "override"})
	e.removeOwnerRegistrations(e.owner)
	if !e.actions["command.quit"] {
		t.Fatal("removing an override removed the built-in action")
	}
}

func TestOwnerBindingsUseCanonicalKeysAndRetainHiddenCallbacks(t *testing.T) {
	e := ownershipFixture()
	id, err := bindingIdentity("prompt.normal", "<C-m>")
	if err != nil {
		t.Fatal(err)
	}
	alias, err := bindingIdentity("prompt.normal", "<enter>")
	if err != nil || alias != id {
		t.Fatalf("equivalent key identities: %q %q %v", id, alias, err)
	}
	e.setOwnedBinding(id, keymap.Binding{Mode: "prompt.normal", Keys: "<C-m>", Action: "base"})
	e.owner = "extension"
	e.setOwnedBinding(alias, keymap.Binding{Mode: "prompt.normal", Keys: "<enter>", Action: "overlay"})
	retained := map[string]bool{}
	e.retainOwnerCallbacks(retained)
	if !retained["base"] || !retained["overlay"] {
		t.Fatal("covered callback was not retained")
	}
	e.removeOwnerRegistrations("extension")
	registry, err := keymap.New(e.manifest.Config.Keymaps)
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Lookup("prompt.normal", []string{"enter"}); got.Action != "base" {
		t.Fatalf("original mapping not restored: %+v", got)
	}
}

func TestExplicitBindingChangesSurvivePackageRemoval(t *testing.T) {
	for _, change := range []string{"replace", "remove", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			e := ownershipFixture()
			binding := keymap.Binding{Mode: "prompt.normal", Keys: "x", Action: "base"}
			id, _ := bindingIdentity(binding.Mode, binding.Keys)
			e.setOwnedBinding(id, binding)
			e.owner = "extension"
			binding.Action = "extension"
			e.setOwnedBinding(id, binding)
			next := slices.Clone(e.manifest.Config.Keymaps)
			want := "base"
			if change == "replace" {
				next[0].Action, want = "user", "user"
			} else if change == "remove" {
				next, want = nil, ""
			}
			e.syncOwnerBindings(next, "config")
			e.manifest.Config.Keymaps = next
			e.removeOwnerRegistrations("extension")
			registry, err := keymap.New(e.manifest.Config.Keymaps)
			if err != nil {
				t.Fatal(err)
			}
			if got := registry.Lookup("prompt.normal", []string{"x"}); got.Action != want {
				t.Fatalf("%s restored incorrect mapping: got %q want %q", change, got.Action, want)
			}
		})
	}
}

func TestInvalidMappingsDoNotConsumeCallbackSlots(t *testing.T) {
	r := openTestRuntime(t, `
for i = 1, 600 do
  assert(not pcall(wraith.keymap.set, "invalid-mode", "x", function() end))
end
wraith.keymap.set("prompt.normal", "x", function() wraith.notify("valid") end)`)
	m := r.Manifest()
	if len(m.Callbacks) != 1 || len(m.Config.Keymaps) != 1 {
		t.Fatalf("invalid mappings retained callbacks: %d callbacks, %d mappings", len(m.Callbacks), len(m.Config.Keymaps))
	}
}
