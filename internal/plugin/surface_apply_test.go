package plugin

import (
	"strings"
	"testing"
)

func TestSurfaceBufferEditsUseObservedRevisionsAndPreserveOwnership(t *testing.T) {
	store := newSurfaceStore("fixture")
	id, err := store.Reserve("config", "buffer")
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Apply(SurfaceMutation{Operation: "buffer.create", ID: id, Owner: "config", Lease: store.OwnerLease("config"), Buffer: &SurfaceBuffer{Lines: []string{"界 hello"}}})
	if err != nil || created.Buffer.Revision != 1 {
		t.Fatalf("create: %+v %v", created, err)
	}
	foreign, err := store.registerOwner("package")
	if err != nil {
		t.Fatal(err)
	}
	mutation := SurfaceMutation{Operation: "buffer.set_lines", ID: id, Owner: "package", Lease: foreign, Expected: 1, Start: 0, End: 1, Lines: []string{"shared edit"}}
	changed, err := store.Apply(mutation)
	if err != nil || changed.Buffer.Revision != 2 || changed.Buffer.Owner != "config" {
		t.Fatalf("shared edit: %+v %v", changed, err)
	}
	if _, err := store.Apply(mutation); err == nil || !strings.Contains(err.Error(), "expected revision 1, current 2") {
		t.Fatalf("stale edit: %v", err)
	}
	mutation.Expected = 2
	repeated, err := store.Apply(mutation)
	if err != nil || repeated.Buffer.Revision != 2 {
		t.Fatalf("no-op edit changed revision: %+v %v", repeated, err)
	}
	mutation.Lines = nil
	emptied, err := store.Apply(mutation)
	if err != nil || len(emptied.Buffer.Lines) != 1 || emptied.Buffer.Lines[0] != "" {
		t.Fatalf("empty buffer: %+v %v", emptied, err)
	}
	store.RevokeOwner("package")
	if _, err := store.Apply(mutation); err == nil {
		t.Fatal("revoked receipt mutated foreign buffer")
	}
	if _, err := store.Buffer(id); err != nil {
		t.Fatal("foreign buffer disposed with its last editor")
	}
}

func TestSurfaceTextAndSpansRejectControlsWithoutPartialEdits(t *testing.T) {
	store := newSurfaceStore("fixture")
	id, err := store.Reserve("config", "buffer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.HostApply(SurfaceMutation{Operation: "buffer.create", ID: id, Buffer: &SurfaceBuffer{Lines: []string{"界 hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"\x1b[2J", "bad\xff", "embedded\nline", strings.Repeat("x", (64<<10)+1)} {
		_, err := store.HostApply(SurfaceMutation{Operation: "buffer.set_lines", ID: id, Expected: 1, End: 1, Lines: []string{line}})
		if err == nil {
			t.Fatalf("invalid display text accepted: %q", line[:min(len(line), 32)])
		}
	}
	invalid := []SurfaceSpan{
		{Start: SurfacePosition{0, 0}, End: SurfacePosition{0, 8}},
		{Start: SurfacePosition{0, 1}, End: SurfacePosition{0, 0}},
		{Start: SurfacePosition{0, 0}, End: SurfacePosition{0, 1}, Style: SurfaceStyle{Foreground: "\x1b[31m"}},
	}
	for _, span := range invalid {
		if _, err := store.HostApply(SurfaceMutation{Operation: "buffer.set_spans", ID: id, Expected: 1, Spans: []SurfaceSpan{span}}); err == nil {
			t.Fatalf("invalid span accepted: %+v", span)
		}
	}
	valid := SurfaceSpan{Start: SurfacePosition{0, 0}, End: SurfacePosition{0, 1}, Style: SurfaceStyle{Foreground: "accent", Bold: true}}
	changed, err := store.HostApply(SurfaceMutation{Operation: "buffer.set_spans", ID: id, Expected: 1, Spans: []SurfaceSpan{valid}})
	if err != nil || changed.Buffer.Revision != 2 {
		t.Fatalf("valid rune span: %+v %v", changed, err)
	}
	repeated, err := store.HostApply(SurfaceMutation{Operation: "buffer.set_spans", ID: id, Expected: 2, Spans: []SurfaceSpan{valid}})
	if err != nil || repeated.Buffer.Revision != 2 {
		t.Fatalf("unchanged spans bumped revision: %+v %v", repeated, err)
	}
	store.windows["view"] = SurfaceWindow{ID: "view", Owner: "config", BufferID: id}
	deleted, err := store.HostApply(SurfaceMutation{Operation: "buffer.delete", ID: id, Expected: 2})
	if err != nil || len(deleted.RemovedWindows) != 1 {
		t.Fatalf("buffer disposal orphaned a view: %+v %v", deleted, err)
	}
}
