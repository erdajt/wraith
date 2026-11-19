package plugin

import (
	"testing"

	"github.com/erdajt/wraith/internal/keymap"
)

func TestSurfaceKeymapsRestoreOwnedOverridesAndKeepHiddenCallbacks(t *testing.T) {
	store := newSurfaceStore("maps")
	id, _ := store.Reserve("config", "buffer")
	_, _ = store.Apply(SurfaceMutation{Operation: "buffer.create", ID: id, Owner: "config", Lease: store.OwnerLease("config"), Buffer: &SurfaceBuffer{Lines: []string{"text"}}})
	for i, owner := range []string{"first", "second"} {
		lease, _ := store.registerOwner(owner)
		binding := keymap.Binding{Mode: "buffer.normal", Keys: "x", Action: "lua.callback." + owner}
		_, err := store.Apply(SurfaceMutation{Operation: "buffer.set_keymap", ID: id, Owner: owner, Lease: lease, Expected: uint64(i + 1), Binding: &binding})
		if err != nil {
			t.Fatal(err)
		}
	}
	retained := store.callbackReferences()
	if !retained["lua.callback.first"] || !retained["lua.callback.second"] {
		t.Fatalf("hidden callback was lost: %+v", retained)
	}
	preview := store.snapshot()
	store.RevokeOwner("second")
	current, _ := store.Buffer(id)
	before, _ := preview.Buffer(id)
	if len(current.Keymaps) != 1 || current.Keymaps[0].Action != "lua.callback.first" || before.Keymaps[0].Action != "lua.callback.second" {
		t.Fatalf("override restoration: live=%+v preview=%+v", current.Keymaps, before.Keymaps)
	}
	binding := keymap.Binding{Mode: "buffer.normal", Keys: "x"}
	_, err := store.Apply(SurfaceMutation{Operation: "buffer.del_keymap", ID: id, Owner: "config", Lease: store.OwnerLease("config"), Expected: current.Revision, Binding: &binding})
	if err != nil {
		t.Fatal(err)
	}
	current, _ = store.Buffer(id)
	if len(current.Keymaps) != 0 {
		t.Fatal("deletion tombstone did not hide prior mapping")
	}
	store.RevokeOwner("first")
	if store.callbackReferences()["lua.callback.first"] {
		t.Fatal("unloaded hidden callback retained")
	}
}
