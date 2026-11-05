package terminal

import (
	"strings"
	"testing"
)

func text(s *Screen) string {
	var b strings.Builder
	for _, r := range s.Cells {
		for _, c := range r {
			if c.Text == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c.Text)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestCursorEraseAndAlternateScreen(t *testing.T) {
	s := NewScreen(10, 3, 10)
	s.Feed([]byte("hello\r\x1b[2CXX\x1b[K"))
	if !strings.HasPrefix(text(s), "heXX      ") {
		t.Fatalf("bad erase: %q", text(s))
	}
	s.Feed([]byte("\x1b[?1049hother"))
	if !strings.HasPrefix(text(s), "other") {
		t.Fatal("alternate buffer missing")
	}
	s.Feed([]byte("\x1b[?1049l"))
	if !strings.HasPrefix(text(s), "heXX") {
		t.Fatal("main screen lost")
	}
}

func TestPartialUTF8AndEscapeSequences(t *testing.T) {
	s := NewScreen(12, 3, 10)
	data := []byte("\x1b[38;2;1;2;3m界é\x1b[0m\x1b]52;c;ignored\a!")
	for _, b := range data {
		s.Feed([]byte{b})
	}
	if s.Cells[0][0].Width != 2 || s.Cells[0][0].Style.FG != "38;2;1;2;3" || s.Cells[0][2].Text != "é" || s.Cells[0][3].Text != "!" {
		t.Fatalf("bad unicode or sgr: %+v", s.Cells[0])
	}
	if strings.Contains(strings.Join(s.Lines(false, "", ""), "\n"), "52;") {
		t.Fatal("osc escaped the emulator")
	}
}

func TestScrollbackResizeAndReports(t *testing.T) {
	s := NewScreen(6, 2, 2)
	s.Feed([]byte("one\r\ntwo\r\nthree\r\nfour"))
	if len(s.History) != 2 {
		t.Fatalf("history: %d", len(s.History))
	}
	s.Resize(3, 1)
	s.Feed([]byte("\x1b[999;999H"))
	if s.X != 2 || s.Y != 0 {
		t.Fatal("cursor out of bounds")
	}
	if got := string(s.Feed([]byte("\x1b[6n"))); got != "\x1b[1;3R" {
		t.Fatalf("cursor report: %q", got)
	}
	s.Scroll(20)
	if s.Offset != 2 {
		t.Fatal("unbounded scroll")
	}
}

func FuzzScreen(f *testing.F) {
	f.Add([]byte("hello\x1b[31mworld\x1b[0m"))
	f.Add([]byte("\x1b[?1049h\x1b[99;99H界\x1b[?1049l"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		s := NewScreen(20, 8, 20)
		s.Feed(data)
		s.Resize(7, 3)
		s.Feed(data)
		_ = s.Lines(true, "", "")
		if s.X < 0 || s.Y < 0 || s.X >= s.Width || s.Y >= s.Height {
			t.Fatal("cursor escaped screen")
		}
	})
}

func TestStyleChangePreservesPendingWrap(t *testing.T) {
	s := NewScreen(3, 2, 0)
	s.Feed([]byte("abc\x1b[31md"))
	if s.Cells[0][2].Text != "c" || s.Cells[1][0].Text != "d" {
		t.Fatalf("sgr overwrote right margin: %q", text(s))
	}
	s.Feed([]byte("\x1b[-900A\x1b[-900D"))
	if s.X < 0 || s.Y < 0 || s.X >= s.Width || s.Y >= s.Height {
		t.Fatal("negative parameters escaped screen")
	}
}

func TestWideGlyphCombiningAndResize(t *testing.T) {
	s := NewScreen(4, 2, 0)
	s.Feed([]byte("界\u0301"))
	if s.Cells[0][0].Text != "界\u0301" {
		t.Fatal("combining mark lost its base")
	}
	s.Resize(1, 2)
	if s.Cells[0][0].Text != "" {
		t.Fatal("wide glyph survived inside a single column")
	}
}
