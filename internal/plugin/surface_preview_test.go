package plugin

import "testing"

func TestSurfacePreviewKeepsObservedRevisionUntilNextCallback(t *testing.T) {
	store := newSurfaceStore("preview")
	id, _ := store.Reserve("config", "buffer")
	lease := store.OwnerLease("config")
	_, err := store.Apply(SurfaceMutation{Operation: "buffer.create", ID: id, Owner: "config", Lease: lease, Buffer: &SurfaceBuffer{Lines: []string{"observed"}}})
	if err != nil {
		t.Fatal(err)
	}
	preview := store.snapshot()
	_, err = store.HostApply(SurfaceMutation{Operation: "buffer.set_lines", ID: id, Expected: 1, Start: 0, End: 1, Lines: []string{"user edit"}})
	if err != nil {
		t.Fatal(err)
	}
	observed, _ := preview.Buffer(id)
	if observed.Revision != 1 || observed.Lines[0] != "observed" {
		t.Fatalf("preview changed under callback: %+v", observed)
	}
	mutation := SurfaceMutation{Operation: "buffer.set_lines", ID: id, Owner: "config", Lease: lease, Expected: observed.Revision, Start: 0, End: 1, Lines: []string{"callback edit"}}
	if _, err := preview.Apply(mutation); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Apply(mutation); err == nil {
		t.Fatal("callback overwrote a newer user edit")
	}
	fresh, _ := store.snapshot().Buffer(id)
	if fresh.Revision != 2 || fresh.Lines[0] != "user edit" {
		t.Fatalf("new callback missed committed edit: %+v", fresh)
	}
}

func TestSurfacePreviewDecorationOwnershipDoesNotAliasLiveStore(t *testing.T) {
	store := newSurfaceStore("preview")
	_, _ = store.registerOwner("first")
	_, _ = store.registerOwner("second")
	id, _ := store.Reserve("config", "buffer")
	_, _ = store.Apply(SurfaceMutation{Operation: "buffer.create", ID: id, Owner: "config", Lease: store.OwnerLease("config"), Buffer: &SurfaceBuffer{Lines: []string{"text"}}})
	for i, owner := range []string{"first", "second"} {
		_, err := store.Apply(SurfaceMutation{Operation: "buffer.set_spans", ID: id, Owner: owner, Lease: store.OwnerLease(owner), Expected: uint64(i + 1), Spans: []SurfaceSpan{{Start: SurfacePosition{0, i}, End: SurfacePosition{0, i + 1}}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	preview := store.snapshot()
	store.RevokeOwner("first")
	before, _ := preview.Buffer(id)
	after, _ := store.Buffer(id)
	if len(before.Spans) != 2 || len(after.Spans) != 1 || before.Spans[0].Start.Column != 0 {
		t.Fatalf("decoration layers alias: before=%+v after=%+v", before.Spans, after.Spans)
	}
	preview.RevokeOwner("second")
	after, _ = store.Buffer(id)
	if len(after.Spans) != 1 {
		t.Fatal("preview disposal changed live store")
	}
}
