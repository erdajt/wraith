package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/vim"
	"github.com/mattn/go-runewidth"
)

func (a *App) moveChatText(key string, count int) {
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
}
