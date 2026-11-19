package variables

import "testing"

// TestSnapshotEqualIncludesRetainedPrivacy checks active, historical and marker-only differences.
func TestSnapshotEqualIncludesRetainedPrivacy(t *testing.T) {
	entry := Entry{Name: "TARGET", Value: "private.example"}
	first, second := NewSnapshot([]Entry{entry}, nil), NewSnapshot([]Entry{entry}, nil)
	if !first.Equal(second) || !first.Equal(first) {
		t.Fatal("identical immutable snapshots differ")
	}
	first.Redact(entry.Value)
	if !first.Equal(second) {
		t.Fatal("lazy matcher initialization changed privacy equality")
	}
	if first.Equal(NewSnapshot([]Entry{entry}, []Entry{{Name: "OLD", Value: "old.example"}})) {
		t.Fatal("retained private value was ignored")
	}
	if (Snapshot{}).Equal(NewSnapshot(nil, []Entry{{Name: "EMPTY"}})) {
		t.Fatal("retained marker name was ignored")
	}
	entry.Description = "changed metadata"
	if first.Equal(NewSnapshot([]Entry{entry}, nil)) {
		t.Fatal("binding metadata was ignored")
	}
}
