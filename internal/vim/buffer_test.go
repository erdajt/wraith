package vim

import "testing"

func TestBufferUndoRestoresUnicode(t *testing.T) {
	b := New()
	b.Set("界a")
	b.Insert("b")
	b.restore(false)
	if b.String() != "界a" || b.Pos != 2 {
		t.Fatalf("undo: %q at %d", b.String(), b.Pos)
	}
	b.restore(true)
	if b.String() != "界ab" {
		t.Fatal("redo lost inserted text")
	}
	b.Mode = "visual"
	b.Anchor = 0
	b.Pos = 1
	a, z := b.Selection()
	if a != 0 || z != 2 {
		t.Fatal("selection must use rune positions")
	}
}
