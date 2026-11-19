package plugin

import "testing"

func TestSurfaceReservationsAndOwnerLeases(t *testing.T) {
	store := newSurfaceStore("fixture")
	first, err := store.Reserve("config", "buffer")
	if err != nil {
		t.Fatal(err)
	}
	store.DiscardReservations([]string{first})
	second, err := store.Reserve("config", "buffer")
	if err != nil || first == second {
		t.Fatalf("reservation reused identity: %q %q %v", first, second, err)
	}
	mutation := SurfaceMutation{Owner: "config", Lease: store.OwnerLease("config")}
	if !store.Valid(mutation) {
		t.Fatal("active owner rejected")
	}
	for range 63 {
		if _, err := store.Reserve("config", "buffer"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Reserve("config", "buffer"); err == nil {
		t.Fatal("reservation capacity ignored")
	}
	store.RevokeOwner("config")
	if store.Valid(mutation) || len(store.reserved) != 0 {
		t.Fatal("revoked owner retained work")
	}
	if _, err := store.registerOwner("config"); err == nil {
		t.Fatal("revoked package generation revived")
	}
	store.close()
	if store.OwnerValid("core", store.OwnerLease("core")) {
		t.Fatal("closed store retained a valid owner")
	}
}

func TestSurfaceOwnerDisposalPreservesForeignTextAndRestoresDecorations(t *testing.T) {
	store := newSurfaceStore("fixture")
	if _, err := store.registerOwner("package"); err != nil {
		t.Fatal(err)
	}
	base := SurfaceSpan{Style: SurfaceStyle{Foreground: "accent"}}
	overlay := SurfaceSpan{Style: SurfaceStyle{Foreground: "warn"}}
	store.buffers["shared"] = &surfaceBufferRecord{
		SurfaceBuffer: SurfaceBuffer{ID: "shared", Owner: "config", Revision: 2, Lines: []string{"changed text"}, Spans: []SurfaceSpan{base, overlay}},
		decorations:   []surfaceDecoration{{owner: "config", spans: []SurfaceSpan{base}}, {owner: "package", spans: []SurfaceSpan{overlay}}},
	}
	store.buffers["owned"] = &surfaceBufferRecord{SurfaceBuffer: SurfaceBuffer{ID: "owned", Owner: "package"}}
	store.windows["foreign-view"] = SurfaceWindow{ID: "foreign-view", Owner: "config", BufferID: "owned"}
	store.windows["owned-view"] = SurfaceWindow{ID: "owned-view", Owner: "package", BufferID: "shared"}
	store.regions["region"] = SurfaceRegion{ID: "region", Owner: "package"}
	delta := store.RevokeOwner("package")
	if len(delta.RemovedBuffers) != 1 || len(delta.RemovedWindows) != 2 || len(delta.RemovedRegions) != 1 || len(delta.ChangedBuffers) != 1 {
		t.Fatalf("incomplete disposal: %+v", delta)
	}
	shared := delta.ChangedBuffers[0]
	if shared.Lines[0] != "changed text" || shared.Revision != 3 || len(shared.Spans) != 1 || shared.Spans[0] != base {
		t.Fatalf("foreign text or original decoration lost: %+v", shared)
	}
	shared.Lines[0] = "mutated snapshot"
	if store.buffers["shared"].Lines[0] != "changed text" {
		t.Fatal("disposal result aliases live text")
	}
}
