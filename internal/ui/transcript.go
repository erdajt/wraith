package ui

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"github.com/mattn/go-runewidth"
	"strings"
	"unicode"
)

type rendered struct {
	Text    string
	Message int
}

func (a *App) transcript(w int) []rendered {
	p := palettes[a.Config.Theme]
	var out []rendered
	add := func(text string, index int) {
		for _, line := range strings.Split(ansi.Hardwrap(text, w, true), "\n") {
			out = append(out, rendered{line, index})
		}
	}
	if a.Session.Summary != "" {
		add(ink(p.Second, "◌ compacted memory · /archives restores originals"), -1)
		add(ink(p.Muted, clean(a.Session.Summary)), -1)
		add("", -1)
	}
	messages := append([]session.Message(nil), a.Session.Messages...)
	if a.busy && !a.compacting {
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
		if a.showThinking && m.Thinking != "" {
			add(ink(p.Muted, "thinking: "+clean(m.Thinking)), i)
		}
		inCode := false
		fence := ""
		for _, line := range strings.Split(clean(m.Content), "\n") {
			trim := strings.TrimSpace(line)
			if !inCode && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")) {
				inCode = true
				fence = trim[:3]
				block++
				lang := strings.TrimSpace(trim[3:])
				add(ink(p.Border, "╭─ ")+ink(p.Second, fmt.Sprintf("%s · block %d", lang, block))+ink(p.Muted, " · /blocks to edit"), i)
				continue
			}
			if inCode && strings.HasPrefix(trim, fence) {
				inCode = false
				add(ink(p.Border, "╰"+strings.Repeat("─", min(w-1, 40))), i)
				continue
			}
			if inCode {
				add(ink(p.Border, "│ ")+ink(p.Accent, line), i)
			} else if strings.HasPrefix(trim, "#") {
				add(bold(p.FG, strings.TrimLeft(trim, "# ")), i)
			} else {
				add(ink(p.FG, line), i)
			}
		}
		add("", i)
	}
	return out
}

func (a *App) promptLines(w int) []string {
	p := palettes[a.Config.Theme]
	b := a.Prompt
	var lines []string
	var line strings.Builder
	column := 0
	cursorRow := 0
	left, right := b.Selection()
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
				line.WriteString("\x1b[7m " + reset)
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
			line.WriteString("\x1b[7m" + s + reset)
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
