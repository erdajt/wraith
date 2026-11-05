package vim

import "testing"

func TestParagraphMotionsKeepBlankBoundaries(t *testing.T) {
	b := New()
	b.Set("one\ntwo\n\nthree\n\nfour")
	b.Mode = "normal"
	b.Pos = 0
	b.Key("}", "")
	if b.Pos != 8 {
		t.Fatalf("next paragraph: %d", b.Pos)
	}
	b.Key("}", "")
	if b.Pos != 15 {
		t.Fatalf("second paragraph: %d", b.Pos)
	}
	b.Key("{", "")
	if b.Pos != 8 {
		t.Fatalf("previous paragraph: %d", b.Pos)
	}
}
