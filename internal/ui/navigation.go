package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func (a *App) jumpSelected() {
	for i, line := range a.transcript(max(10, a.mainWidth()-5)) {
		if line.Message == a.selected {
			a.offset = i
			a.chatCursor = i
			a.chatColumn = 0
			a.follow = false
			return
		}
	}
}
func (a *App) findNext(reverse bool) {
	if a.search == "" {
		return
	}
	lines := a.transcript(max(10, a.mainWidth()-5))
	if len(lines) == 0 {
		return
	}
	type match struct{ row, column int }
	var matches []match
	for row, line := range lines {
		runes := []rune(ansi.Strip(line.Text))
		needle := []rune(a.search)
		for i := 0; i+len(needle) <= len(runes); i++ {
			if strings.EqualFold(string(runes[i:i+len(needle)]), a.search) {
				matches = append(matches, match{row, runewidth.StringWidth(string(runes[:i]))})
			}
		}
	}
	if len(matches) == 0 {
		a.notice = "no match: " + a.search
		return
	}
	a.ensureChatCursor()
	chosen := matches[0]
	if reverse {
		chosen = matches[len(matches)-1]
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			if m.row < a.chatCursor || m.row == a.chatCursor && m.column < a.chatColumn {
				chosen = m
				break
			}
		}
	} else {
		for _, m := range matches {
			if m.row > a.chatCursor || m.row == a.chatCursor && m.column > a.chatColumn {
				chosen = m
				break
			}
		}
	}
	a.chatCursor, a.chatColumn = chosen.row, chosen.column
	a.selected = lines[chosen.row].Message
	a.revealCursor(len(lines))
	a.notice = "match: " + a.search
}
