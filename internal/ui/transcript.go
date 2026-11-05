package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/markdown"
	"github.com/erdajt/wraith/internal/session"
	"github.com/mattn/go-runewidth"
)

type rendered struct {
	Block    int
	Thinking bool
	Text     string
	Message  int
}

func (a *App) transcript(w int) []rendered {
	p := a.Config.Palette()
	var out []rendered
	add := func(text string, index int) {
		for _, line := range strings.Split(ansi.Hardwrap(text, w, true), "\n") {
			out = append(out, rendered{Text: line, Message: index, Block: -1})
		}
	}
	if a.Session.Summary != "" {
		add(ink(p.Second, "◌ compacted memory · /archives restores originals"), -1)
		add(ink(p.Muted, clean(a.Session.Summary)), -1)
		add("", -1)
	}
	messages := append([]session.Message(nil), a.Session.Messages...)
	if a.busy && !a.compacting && !a.executing {
		messages = append(messages, a.partial)
	}
	block := 0
	for i, m := range messages {
		label := "YOU"
		c := p.Accent
		if m.Role == "assistant" {
			label = "WRAITH"
			c = p.Second
		}
		marker := "◇"
		if a.focus == "chat" && a.selected == i {
			marker = "◆"
		}
		meta := ""
		if m.Edited {
			meta += " · edited"
		}
		if m.Interrupted {
			meta += " · interrupted"
		}
		add(bold(c, marker+" "+label)+ink(p.Muted, fmt.Sprintf("  %02d%s", i+1, meta)), i)
		if m.Thinking != "" {
			expanded := a.thinkingOpen(i)
			arrow := "▸"
			if expanded {
				arrow = "▾"
			}
			start := len(out)
			for _, line := range strings.Split(ansi.Hardwrap(arrow+" thinking · "+a.Config.Keys["thinking_toggle"]+" or click to toggle", w, true), "\n") {
				add(ink(p.Muted, line), i)
			}
			for j := start; j < len(out); j++ {
				out[j].Thinking = true
			}
			if expanded {
				for _, line := range strings.Split(ansi.Hardwrap(clean(m.Thinking), w, true), "\n") {
					add(ink(p.Muted, line), i)
				}
			}
		}

		lines, nextBlock := markdown.Render(clean(m.Content), w, p, block)
		block = nextBlock
		for _, line := range lines {
			start := len(out)
			add(line.Text, i)
			for j := start; j < len(out); j++ {
				out[j].Block = line.Block
			}
		}
		add("", i)
	}
	return out
}

func (a *App) promptLines(w int) []string {
	p := a.Config.Palette()
	b := a.Prompt
	var lines []string
	var line strings.Builder
	column := 0
	cursorRow := 0
	left, right := b.Selection()
	matches := make([]bool, len(b.Text))
	needle := []rune(b.SearchQuery())
	if len(needle) > 0 {
		for i := 0; i+len(needle) <= len(b.Text); i++ {
			if strings.EqualFold(string(b.Text[i:i+len(needle)]), string(needle)) {
				for j := i; j < i+len(needle); j++ {
					matches[j] = true
				}
			}
		}
	}
	for i := 0; i <= len(b.Text); i++ {
		r := ' '
		if i < len(b.Text) {
			r = b.Text[i]
		}
		cursor := a.focus == "prompt" && !a.commandMode && !a.searchMode && i == b.Pos
		if i == len(b.Text) && !cursor {
			break
		}
		if column >= w {
			lines = append(lines, line.String())
			line.Reset()
			column = 0
		}
		if cursor {
			cursorRow = len(lines)
		}
		if r == '\n' {
			if cursor {
				line.WriteString(cursorText(" ", 0, p))
			}
			lines = append(lines, line.String())
			line.Reset()
			column = 0
			continue
		}
		s := string(r)
		if r == '\t' {
			s = "    "
		}
		if unicode.IsControl(r) {
			s = ""
		}
		width := runewidth.StringWidth(s)
		if column+width > w {
			lines = append(lines, line.String())
			line.Reset()
			column = 0
			if cursor {
				cursorRow = len(lines)
			}
		}
		if cursor || (i >= left && i < right) {
			line.WriteString(cursorText(s, 0, p))
		} else if i < len(matches) && matches[i] {
			line.WriteString(highlightRange(s, 0, width, p.Warn, p))
		} else {
			line.WriteString(ink(p.FG, s))
		}
		column += width
	}
	lines = append(lines, line.String())
	start := max(0, cursorRow-3)
	if start >= len(lines) {
		start = 0
	}
	return lines[start:min(len(lines), start+4)]
}
