package plugin

import "testing"

// TestNativeProjectionRevisionsAndWriteGuards protects canonical conversation and incomplete drafts.
func TestNativeProjectionRevisionsAndWriteGuards(t *testing.T) {
	s := newSurfaceStore("native")
	first, err := s.HostUpsert("prompt", []string{"draft"}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.HostUpsert("prompt", []string{"draft"}, false)
	if err != nil || second.Buffer.ID != first.Buffer.ID || second.Buffer.Revision != first.Buffer.Revision {
		t.Fatalf("unchanged projection: %+v, %v", second, err)
	}
	mutation := SurfaceMutation{Operation: "buffer.set_lines", ID: first.Buffer.ID, Owner: "config", Lease: s.OwnerLease("config"), Expected: first.Buffer.Revision, End: 1, Lines: []string{"edited"}}
	edited, err := s.Apply(mutation)
	if err != nil || edited.Buffer.Lines[0] != "edited" {
		t.Fatalf("explicit prompt edit: %+v, %v", edited, err)
	}
	if _, err := s.Apply(mutation); err == nil {
		t.Fatal("stale edit accepted")
	}
	partial, err := s.HostUpsert("prompt", []string{"partial"}, true)
	if err != nil || partial.Buffer.Editable {
		t.Fatalf("partial draft: %+v, %v", partial, err)
	}
	mutation.Expected = partial.Buffer.Revision
	if _, err := s.Apply(mutation); err == nil {
		t.Fatal("incomplete draft was writable")
	}
	for _, slot := range []string{"prompt", "conversation"} {
		delta, err := s.HostUpsert(slot, []string{"native"}, false)
		if err != nil {
			t.Fatal(err)
		}
		mutation.ID, mutation.Expected = delta.Buffer.ID, delta.Buffer.Revision
		mutation.Operation = "buffer.delete"
		if _, err := s.Apply(mutation); err == nil {
			t.Fatal("native projection deleted")
		}
		if slot == "conversation" {
			mutation.Operation = "buffer.set_lines"
			if _, err := s.Apply(mutation); err == nil {
				t.Fatal("conversation projection changed")
			}
		}
		mutation.Operation = "buffer.set_spans"
		mutation.Spans = []SurfaceSpan{{Start: SurfacePosition{}, End: SurfacePosition{Column: 1}, Style: SurfaceStyle{Bold: true}}}
		if _, err := s.Apply(mutation); err != nil {
			t.Fatalf("native decoration rejected: %v", err)
		}
	}
}
