package plugin

import "testing"

func TestSurfaceTextRevisionIgnoresPresentationChanges(t *testing.T) {
	store := newSurfaceStore("text-revision")
	id, _ := store.Reserve("config", "buffer")
	lease := store.OwnerLease("config")
	_, _ = store.Apply(SurfaceMutation{Operation: "buffer.create", ID: id, Owner: "config", Lease: lease, Buffer: &SurfaceBuffer{Lines: []string{"text"}}})
	_, err := store.Apply(SurfaceMutation{Operation: "buffer.set_spans", ID: id, Owner: "config", Lease: lease, Expected: 1, Spans: []SurfaceSpan{{Start: SurfacePosition{}, End: SurfacePosition{0, 1}}}})
	if err != nil {
		t.Fatal(err)
	}
	buffer, _ := store.Buffer(id)
	if buffer.Revision != 2 || buffer.TextRevision != 1 || store.Index().Buffers[0].TextRevision != 1 {
		t.Fatalf("decoration changed text revision: %+v", buffer)
	}
	_, _ = store.Apply(SurfaceMutation{Operation: "buffer.set_lines", ID: id, Owner: "config", Lease: lease, Expected: 2, Start: 0, End: 1, Lines: []string{"next"}})
	buffer, _ = store.Buffer(id)
	if buffer.Revision != 3 || buffer.TextRevision != 2 {
		t.Fatalf("text edit missed revision: %+v", buffer)
	}
	delta, _ := store.HostUpsert("prompt", []string{"draft"}, false)
	projection, _ := store.HostUpsert("prompt", []string{"draft"}, true)
	if projection.Buffer.TextRevision != delta.Buffer.TextRevision || projection.Buffer.Revision == delta.Buffer.Revision {
		t.Fatal("projection completeness changed text revision")
	}
}
