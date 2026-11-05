package ui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

type palette struct{ BG, FG, Muted, Accent, Second, Border, Warn string }

var palettes = map[string]palette{
	"phosphor": {"#101419", "#d8e4df", "#788b87", "#73f5bd", "#bb9aff", "#354743", "#ffb86b"},
	"tokyo":    {"#161822", "#c0caf5", "#737ca6", "#7aa2f7", "#bb9af7", "#353b5a", "#e0af68"},
	"ember":    {"#1b1515", "#f0ded3", "#aa8b82", "#ffb454", "#f47c91", "#55403a", "#ff6969"},
	"ice":      {"#10191f", "#d8eff5", "#7195a3", "#76e4f7", "#adbaff", "#304b58", "#ffb8a3"},
	"mono":     {"#151515", "#e0e0e0", "#888888", "#ffffff", "#bcbcbc", "#444444", "#eeeeee"},
}

const reset = "\x1b[0m"

func color(hex string, bg bool) string {
	if len(hex) != 7 {
		return ""
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return ""
	}
	code := 38
	if bg {
		code = 48
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", code, v>>16, (v>>8)&255, v&255)
}
func ink(c, text string) string  { return "\x1b[" + color(c, false) + "m" + text + reset }
func bold(c, text string) string { return "\x1b[1;" + color(c, false) + "m" + text + reset }
func clean(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 {
			return -1
		}
		return r
	}, s)
}
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func base(p palette, s string, w int) string {
	style := "\x1b[0;" + color(p.FG, false) + ";" + color(p.BG, true) + "m"
	return style + strings.ReplaceAll(fit(s, w), reset, style) + reset
}
