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
func keys(b *Buffer, s ...string) {
	for _, k := range s {
		b.Key(k, k)
	}
}
func normal(s string) *Buffer { b := New(); b.Set(s); b.Mode = "normal"; b.Pos = 0; return b }

func TestMotionsOperatorsUndoAndPaste(t *testing.T) {
	b := normal("alpha beta\ngamma delta")
	keys(b, "d", "w")
	if b.String() != "beta\ngamma delta" {
		t.Fatalf("dw: %q", b.String())
	}
	keys(b, "u")
	if b.String() != "alpha beta\ngamma delta" {
		t.Fatal("undo failed")
	}
	keys(b, "w", "c", "i", "w")
	b.Key("runes", "new")
	b.Key("esc", "")
	if b.String() != "alpha new\ngamma delta" {
		t.Fatalf("ciw: %q", b.String())
	}
	keys(b, "g", "g", "y", "y", "G", "p")
	if b.String() != "alpha new\ngamma delta\nalpha new\n" {
		t.Fatalf("line paste: %q", b.String())
	}
}

func TestVisualUnicodeAndSearch(t *testing.T) {
	b := normal("界 alpha beta alpha")
	keys(b, "v", "l", "d")
	if b.String() != "alpha beta alpha" {
		t.Fatalf("visual delete: %q", b.String())
	}
	keys(b, "/", "a", "l", "p", "h", "a")
	b.Key("enter", "")
	if b.Pos != 11 {
		t.Fatalf("search position: %d", b.Pos)
	}
	keys(b, "n")
	if b.Pos != 0 {
		t.Fatal("search did not wrap")
	}
}

func TestMultilineInsertAndCounts(t *testing.T) {
	b := New()
	b.Key("runes", "hello")
	b.Key("enter", "")
	b.Key("runes", "world")
	b.Key("esc", "")
	keys(b, "g", "g", "2", "d", "d")
	if b.String() != "" {
		t.Fatalf("2dd: %q", b.String())
	}
	keys(b, "u")
	if b.String() != "hello\nworld" {
		t.Fatal("undo multiline failed")
	}
}

func FuzzEditing(f *testing.F) {
	f.Add("hello world", "dwiabc\x1bu")
	f.Fuzz(func(t *testing.T, text, seq string) {
		if len(text)+len(seq) > 4096 {
			t.Skip()
		}
		b := normal(text)
		for _, r := range seq {
			b.Key(string(r), string(r))
			if b.Pos < 0 || b.Pos > len(b.Text) {
				t.Fatal("cursor out of bounds")
			}
		}
	})
}
