package ui

import (
	"fmt"
	"github.com/erdajt/wraith/internal/config"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

type palette = config.Palette

var palettes = config.BuiltinThemes()

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
