package vim

import "testing"

func TestViewsShareUndoWithoutSharingNavigation(t *testing.T) {
	first := New()
	first.Text, first.Pos, first.Mode, first.Search = []rune("a界c"), 1, "normal", "界"
	second := first.CloneView()
	second.Pos, second.Search = 2, "c"
	first.Key("i", "")
	first.Key("x", "x")
	second.SyncText(first.String(), false)
	if second.Pos != 3 || second.Mode != "normal" || second.Search != "c" || first.Mode != "insert" {
		t.Fatalf("view state leaked: first=%+v second=%+v", first, second)
	}
	second.Key("u", "")
	first.SyncText(second.String(), false)
	if first.String() != "a界c" || second.String() != "a界c" || first.Mode != "insert" {
		t.Fatalf("shared undo failed: first=%q second=%q", first.String(), second.String())
	}
	second.Key("ctrl+r", "")
	first.SyncText(second.String(), false)
	if first.String() != "ax界c" || second.String() != "ax界c" {
		t.Fatal("shared redo lost the edit")
	}
	second.Text[0] = 'z'
	if first.Text[0] != 'a' {
		t.Fatal("view text storage was aliased")
	}
}

func TestViewSyncRebasesRuneAnchors(t *testing.T) {
	for _, test := range []struct {
		name, before, after string
		position, want      int
	}{
		{"insertion before affinity", "a界b", "ax界b", 1, 1},
		{"insertion shifts later runes", "a界b", "ax界b", 2, 3},
		{"deletion clamps overlap", "a界e\u0301z", "az", 3, 1},
		{"deletion shifts retained suffix", "a界e\u0301z", "az", 4, 1},
		{"combining insertion counts runes", "aez", "ae\u0301z", 2, 2},
		{"replacement preserves suffix", "a界z", "alongz", 2, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := New()
			view.Text, view.Pos, view.Mode = []rune(test.before), test.position, "normal"
			view.SyncText(test.after, false)
			if view.Pos != test.want {
				t.Fatalf("cursor = %d, want %d", view.Pos, test.want)
			}
		})
	}
}

func TestExternalReplacementClearsSharedHistoryAndOverlappingSelection(t *testing.T) {
	first := New()
	first.Set("one\n界two\nthree")
	first.Key("esc", "")
	second := first.CloneView()
	second.Mode, second.Anchor, second.Pos = "visual", 4, 7
	second.Pending, second.Count = "f", "3"
	second.SyncText("one\nchanged\nthree", true)
	first.SyncText(second.String(), false)
	if second.Mode != "normal" || second.Pending != "" || second.Count != "" || second.Pos != 4 {
		t.Fatalf("stale selection survived replacement: %+v", second)
	}
	first.Key("u", "")
	if first.String() != "one\nchanged\nthree" {
		t.Fatal("external replacement left stale shared undo")
	}
}

func TestUnchangedTextPreservesSharedUndo(t *testing.T) {
	first := New()
	first.Set("before")
	first.Set("after")
	second := first.CloneView()
	second.SyncText("after", true)
	first.Key("esc", "")
	first.Key("u", "")
	if first.String() != "before" {
		t.Fatal("unchanged text reset document history")
	}
}

// TestNewViewSharesUndoWithoutNavigation keeps document history separate from view state.
func TestNewViewSharesUndoWithoutNavigation(t *testing.T) {
	first := New()
	first.Set("before")
	first.Set("after")
	first.Mode, first.Search, first.query = "visual", "after", "aft"
	first.Anchor, first.Pos = 1, 3
	first.Pending, first.Count, first.OperatorCount = "d", "2", "3"
	first.vertical, first.preferredColumn = true, 9
	first.Register, first.Yanked = []rune("copied"), true

	second := first.NewView()
	if second.Mode != "normal" || second.Pos != 0 || second.Anchor != 0 || second.Search != "" || second.query != "" || second.Pending != "" || second.Count != "" || second.OperatorCount != "" || second.vertical || second.preferredColumn != 0 || len(second.Register) != 0 || second.Yanked {
		t.Fatalf("new view inherited navigation: %+v", second)
	}
	second.Text[0] = 'A'
	if first.String() != "after" {
		t.Fatal("new view aliases document storage")
	}
	second.Key("u", "")
	if second.String() != "before" || first.String() != "after" {
		t.Fatal("new view lost shared undo or mutated the other view")
	}
	first.Key("esc", "")
	first.Key("ctrl+r", "")
	if first.String() != "After" {
		t.Fatalf("redo history was not shared: %q", first.String())
	}
}
