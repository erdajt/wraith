package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func highlightRange(text string, start, end int, colorValue string, p palette) string {
	foreground := p.BG
	if foreground == "" {
		foreground = "#101419"
	}
	style := "\x1b[" + color(foreground, false) + ";" + color(colorValue, true) + "m"
	return ansi.Cut(text, 0, start) + style + ansi.Strip(ansi.Cut(text, start, end)) + reset + ansi.Cut(text, end, ansi.StringWidth(text))
}

func highlightMatches(text, query string, p palette) string {
	needle := []rune(query)
	if len(needle) == 0 {
		return text
	}
	runes := []rune(ansi.Strip(text))
	for i := len(runes) - len(needle); i >= 0; i-- {
		if strings.EqualFold(string(runes[i:i+len(needle)]), query) {
			start := runewidth.StringWidth(string(runes[:i]))
			end := start + runewidth.StringWidth(string(runes[i:i+len(needle)]))
			text = highlightRange(text, start, end, p.Warn, p)
		}
	}
	return text
}
