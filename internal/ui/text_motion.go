package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/vim"
	"github.com/mattn/go-runewidth"
)

func (a *App) moveChatText(key string, count int) tea.Cmd {
	a.ensureChatCursor()
	lines := a.transcript(max(10, a.mainWidth()-5))
	var plain []string
	position := 0
	for i, line := range lines {
		text := ansi.Strip(line.Text)
		plain = append(plain, text)
		if i < a.chatCursor {
			position += len([]rune(text)) + 1
		}
		if i == a.chatCursor {
			cells := 0
			for _, r := range text {
				if cells >= a.chatColumn {
					break
				}
				cells += runewidth.RuneWidth(r)
				position++
			}
		}
	}
	b := vim.New()
	b.Text = []rune(strings.Join(plain, "\n"))
	b.Pos = min(position, len(b.Text))
	b.Mode = "normal"
	b.Pending = a.chatMotionPending
	b.Count = a.chatMotionCount
	if b.Pending == "" && count > 1 {
		b.Count = strconv.Itoa(count)
	}
	b.Key(key, key)
	a.chatMotionPending, a.chatMotionCount = b.Pending, b.Count
	a.chatCursor, a.chatColumn = 0, 0
	for _, r := range b.Text[:b.Pos] {
		if r == '\n' {
			a.chatCursor++
			a.chatColumn = 0
		} else {
			a.chatColumn += runewidth.RuneWidth(r)
		}
	}
	if len(lines) > 0 {
		a.selected = lines[min(a.chatCursor, len(lines)-1)].Message
	}
	a.revealCursor(len(lines))
	if len(b.Register) > 0 {
		first, left := textPosition(b.Text, b.YankStart)
		last, right := textPosition(b.Text, max(b.YankStart, b.YankEnd-1))
		a.visualRow, a.visualColumn = first, left
		savedRow, savedColumn := a.chatCursor, a.chatColumn
		a.chatCursor, a.chatColumn, a.visualMode = last, right, "visual"
		text := a.selectedText()
		a.chatCursor, a.chatColumn = savedRow, savedColumn
		return a.copyText(text, b.Linewise)
	}
	return nil
}

func textPosition(text []rune, end int) (row, column int) {
	for _, r := range text[:min(end, len(text))] {
		if r == '\n' {
			row++
			column = 0
		} else {
			column += runewidth.RuneWidth(r)
		}
	}
	return
}
