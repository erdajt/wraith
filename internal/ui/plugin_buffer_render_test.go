package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/plugin"
)

func TestSurfaceBufferStylesPreserveTextAndReaderHighlights(t *testing.T) {
	p := fixture(t).Config.Palette()
	text := "a界e\u0301z"
	buffer := plugin.SurfaceBuffer{Lines: []string{text}, Spans: []plugin.SurfaceSpan{
		{Start: plugin.SurfacePosition{Line: 0, Column: 1}, End: plugin.SurfacePosition{Line: 0, Column: 4}, Style: plugin.SurfaceStyle{Foreground: "accent", Bold: true}},
		{Start: plugin.SurfacePosition{Line: 0, Column: 2}, End: plugin.SurfacePosition{Line: 0, Column: 4}, Style: plugin.SurfaceStyle{Foreground: "warn", Italic: true}},
	}}
	view := newReader(text)
	view.Buffer.Pos, view.Buffer.Search = 3, "界"
	frame := renderSurfaceBuffer(buffer, view, 10, 2, p, false)
	if ansi.Strip(frame[0]) != text+strings.Repeat(" ", 5) || !strings.Contains(frame[0], color(p.Accent, false)) || !strings.Contains(frame[0], color(p.Warn, true)) {
		t.Fatalf("styles changed text or lost search highlight: %q", frame[0])
	}
	view.Buffer.Mode, view.Buffer.Anchor, view.Buffer.Pos = "visual", 2, 3
	frame = renderSurfaceBuffer(buffer, view, 4, 1, p, true)
	if ansi.StringWidth(frame[0]) != 4 || !strings.Contains(frame[0], color(p.Second, true)) {
		t.Fatalf("wide cursor/selection escaped viewport: %q", frame[0])
	}
	if got := renderSurfaceBuffer(buffer, view, 0, 0, p, true); len(got) != 0 {
		t.Fatalf("zero-sized view produced rows: %v", got)
	}
}

func TestRuneDisplayRangeKeepsCombiningAndJoinedGlyphsTogether(t *testing.T) {
	for _, test := range []struct {
		text                 string
		start, end, from, to int
	}{
		{"a界z", 1, 2, 1, 3},
		{"ae\u0301z", 2, 3, 1, 2},
		{"ae\u0301z", 1, 2, 1, 2},
		{"a👩‍💻z", 2, 3, 1, 3},
		{"a界", 2, 2, 3, 3},
	} {
		from, to := runeDisplayRange(test.text, test.start, test.end)
		if from != test.from || to != test.to {
			t.Fatalf("%q [%d:%d] = [%d:%d], want [%d:%d]", test.text, test.start, test.end, from, to, test.from, test.to)
		}
	}
}

func TestSurfaceStylesExpandToWholeGraphemes(t *testing.T) {
	p := fixture(t).Config.Palette()
	for _, test := range []struct {
		text, glyph string
		start, end  int
	}{
		{"ae\u0301z", "e\u0301", 2, 3},
		{"a👩‍💻z", "👩‍💻", 2, 3},
	} {
		style := plugin.SurfaceStyle{Foreground: "accent"}
		got := styledSurfaceLine(test.text, 0, []plugin.SurfaceSpan{{Start: plugin.SurfacePosition{Column: test.start}, End: plugin.SurfacePosition{Column: test.end}, Style: style}}, p)
		if !strings.Contains(got, surfaceStyle(test.glyph, style, p)) || ansi.StringWidth(got) != ansi.StringWidth(test.text) {
			t.Fatalf("style split a grapheme: %q", got)
		}
	}
}
