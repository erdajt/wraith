package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestKeyHintsUseActionColorsWithoutChangingText(t *testing.T) {
	p := fixture(t).Config.Palette()
	for _, tc := range []struct{ text, color string }{
		{"q quit", p.HintQuit}, {"ctrl+q quit", p.HintQuit}, {"esc close", p.HintQuit},
		{"r refresh", p.HintRefresh}, {"ctrl+x cancel", p.HintCancel}, {"esc rejects", p.HintCancel},
		{"F8 cancel", p.HintCancel}, {"enter select", p.Accent}, {"/ search", p.Accent},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got := keyHints(tc.text, p)
			if ansi.Strip(got) != tc.text || !strings.Contains(got, color(tc.color, false)) {
				t.Fatalf("incorrect hint: %q", got)
			}
		})
	}
	text := "q close · r refresh · ctrl+x cancel · j/k move"
	got := keyHints(text, p)
	if ansi.Strip(got) != text || ansi.StringWidth(got) != ansi.StringWidth(text) {
		t.Fatalf("hint separators changed: %q", got)
	}
}
