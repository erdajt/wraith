package ui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/config"
)

type palette = config.Palette

var palettes = config.BuiltinThemes()

const reset = "\x1b[0m"

// color converts a hexadecimal color to a foreground or background terminal parameter.
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

// ink wraps text in a foreground color followed by a style reset.
func ink(c, text string) string { return "\x1b[" + color(c, false) + "m" + text + reset }

// bold wraps text in bold foreground styling followed by a reset.
func bold(c, text string) string { return "\x1b[1;" + color(c, false) + "m" + text + reset }

// clean removes terminal controls and bidirectional overrides while preserving line breaks.
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

// fit flattens, truncates and pads styled text to a display width.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

// base fills a row and restores its base palette after nested style resets.
func base(p palette, s string, w int) string {
	background := color(p.BG, true)
	if background == "" {
		background = "49"
	}
	style := "\x1b[0;" + color(p.FG, false) + ";" + background + "m"
	return style + strings.ReplaceAll(fit(s, w), reset, style) + reset
}
