package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/markdown"
	"github.com/erdajt/wraith/internal/session"
	"github.com/mattn/go-runewidth"
)

type rendered struct {
	Part         int
	Spans        []markdown.Span
	Block        int
	Thinking     bool
	Text         string
	Message      int
	Section      string
	Source, Cell int
}

// transcript renders conversation rows with message, thinking and code-block metadata.
func (a *App) transcript(w int) []rendered {
	return a.windowTranscript(a.conversationView, w, a.focus == "chat")
}

// windowTranscript renders one view without borrowing another window's navigation state.
func (a *App) windowTranscript(view *conversationView, w int, focused bool) []rendered {
	p := a.Config.Palette()
	if view.surface != nil {
		buffer := a.surfaceBuffers[view.surface.window.BufferID]
		rows := make([]rendered, len(buffer.Lines))
		for i, line := range buffer.Lines {
			rows[i] = rendered{Text: styledSurfaceLine(line, i, buffer.Spans, p), Message: -1, Block: -1, Section: "buffer", Source: i}
		}
		view.rows = rows
		return rows
	}
	var out []rendered
	section, sourceLine, sourceCell := "summary-header", 0, 0
	add := func(text string, index int) {
		cell := sourceCell
		for _, line := range strings.Split(ansi.Hardwrap(text, w, true), "\n") {
			if section == "thinking" {
				line = ink(p.Muted, line)
			}
			out = append(out, rendered{Text: line, Message: index, Block: -1, Section: section, Source: sourceLine, Cell: cell})
			cell += ansi.StringWidth(line)
		}
	}

	if a.Session.Summary != "" {
		add(ink(p.Second, "◌ compacted memory · ")+keyHints("/archives restores originals", p), -1)
		section = "summary"
		for i, line := range strings.Split(a.expandStyled(ink(p.Muted, clean(a.Session.Summary))), "\n") {
			sourceLine = i
			add(line, -1)
		}
		section, sourceLine = "summary-gap", 0
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
		if focused && view.selected == i {
			marker = "◆"
		}

		meta := ""
		if m.Edited {
			meta += " · edited"
		}
		if m.Interrupted {
			meta += " · interrupted"
		}
		section, sourceLine, sourceCell = "header", 0, 0
		add(bold(c, marker+" "+label)+ink(p.Muted, fmt.Sprintf("  %02d%s", i+1, meta)), i)

		if m.Thinking != "" {
			expanded := a.thinkingOpen(i)
			arrow := "▸"
			if expanded {
				arrow = "▾"
			}
			section = "thinking-header"
			start := len(out)
			add(ink(p.Muted, arrow+" thinking · ")+keyHints(a.Config.Keys["thinking_toggle"]+" or click to toggle", p), i)
			for j := start; j < len(out); j++ {
				out[j].Thinking = true
			}

			if expanded {
				section = "thinking"
				for n, line := range strings.Split(a.expandStyled(clean(m.Thinking)), "\n") {
					sourceLine = n
					add(ink(p.Muted, line), i)
				}
			}
		}

		lines, nextBlock := markdown.Render(clean(contextual.DisplayContent(m.Content)), w, p, block, a.expandStyled)
		block = nextBlock
		for _, line := range lines {
			section, sourceLine, sourceCell = "body", line.Source, line.Cell
			if line.Region != "" {
				section += ":" + line.Region
			}
			start := len(out)
			add(line.Text, i)
			for j := start; j < len(out); j++ {
				out[j].Block = line.Block
				out[j].Spans = line.Spans
				out[j].Part = line.Part
			}
		}
		section, sourceLine, sourceCell = "gap", 0, 0
		add("", i)
	}
	view.reflow(out)
	return out
}

// promptLines renders a four-row prompt viewport with cursor, selection and search highlighting.
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
