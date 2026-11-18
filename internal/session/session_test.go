package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSessionRecordsCanonicalDirectoryAndDistinctIdentity(t *testing.T) {
	directory := t.TempDir()
	alias := filepath.Join(t.TempDir(), "project")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	t.Chdir(alias)
	before := time.Now()
	first, second := New("local-model"), New("local-model")
	if first.Directory != directory || first.Model != "local-model" || first.ID == second.ID || !evidenceID.MatchString(first.ID) || !evidenceID.MatchString(second.ID) {
		t.Fatalf("invalid new session identity: %+v", first)
	}
	if first.Created.Before(before) || !first.Created.Equal(first.Updated) || first.Draft != "" || len(first.Messages) != 0 {
		t.Fatal("new session inherited prior state")
	}
}

func TestRetainedRunReferencesRemainIsolatedAcrossClones(t *testing.T) {
	ids := []string{strings.Repeat("a", 24), strings.Repeat("b", 24), strings.Repeat("c", 24), strings.Repeat("d", 24)}
	s := Session{
		Draft: "unsent", Pins: []string{"scope"}, Facts: []string{"fact"}, Archives: []string{"original"},
		RunReferences: []string{ids[0], ids[0], "invalid"}, DeliveredRuns: []string{ids[1]}, SharedRuns: []string{ids[2]},
		Messages: []Message{{Content: "plain"}, {Evidence: &Evidence{ID: ids[3], Stats: &OutputStats{ObservedBytes: 10}}}, {Evidence: &Evidence{ID: "invalid"}}},
	}
	got := s.RunIDs()
	if len(got) != len(ids) {
		t.Fatalf("retained ids lost or invalid ids accepted: %v", got)
	}
	for _, id := range ids {
		if !got[id] {
			t.Fatalf("missing retained id: %s", id)
		}
	}

	clone := s.Clone()
	clone.RunReferences[0], clone.DeliveredRuns[0], clone.SharedRuns[0] = "changed", "changed", "changed"
	clone.Pins[0], clone.Facts[0], clone.Archives[0], clone.Draft = "changed", "changed", "changed", "changed"
	clone.Messages[1].Evidence.Stats.ObservedBytes = 99
	if s.Draft != "unsent" || s.Pins[0] != "scope" || s.Facts[0] != "fact" || s.Archives[0] != "original" || len(s.RunIDs()) != 4 || s.Messages[1].Evidence.Stats.ObservedBytes != 10 {
		t.Fatal("branch mutation changed original session")
	}
}
