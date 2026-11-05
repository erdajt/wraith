package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func cursorText(text string, column int, p palette) string {
	width := ansi.StringWidth(text)
	column = max(0, min(column, max(0, width-1)))
	end := column + 1
	position := 0
	for _, r := range ansi.Strip(text) {
		size := max(0, runewidth.RuneWidth(r))
		if size > 0 && column >= position && column < position+size {
			column = position
			end = position + size
			break
		}
		position += size
	}
	cell := ansi.Cut(text, column, end)
	if strings.TrimSpace(ansi.Strip(cell)) == "" {
		cell = " "
	}
	foreground := p.BG
	if foreground == "" {
		foreground = "#101419"
	}
	style := "\x1b[" + color(foreground, false) + ";" + color(p.Accent, true) + "m"
	return ansi.Cut(text, 0, column) + style + ansi.Strip(cell) + reset + ansi.Cut(text, end, width)
}
