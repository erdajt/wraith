package ui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/editor"
)

type copiedMsg struct{ Err error }

func (a *App) selectionBounds() (int, int, int, int) {
	first, left, last, right := a.visualRow, a.visualColumn, a.chatCursor, a.chatColumn
	if first > last || first == last && left > right {
		first, last = last, first
		left, right = right, left
	}
	return first, left, last, right
}

func (a *App) selectedText() string {
	lines := a.transcript(max(10, a.mainWidth()-5))
	first, left, last, right := a.selectionBounds()
	var out []string
	seen := map[string]bool{}
	for row := max(0, first); row <= last && row < len(lines); row++ {
		entry := lines[row]
		if entry.Block >= 0 {
			id := fmt.Sprintf("%d:%d", entry.Message, entry.Block)
			if !seen[id] {
				blocks := editor.Blocks(a.messageAt(entry.Message))
				if entry.Block < len(blocks) {
					out = append(out, strings.TrimRight(blocks[entry.Block].Text, "\n"))
				}
				seen[id] = true
			}
			continue
		}
		text := ansi.Strip(entry.Text)
		start, end := 0, ansi.StringWidth(text)
		if a.visualMode == "visual" {
			if row == first {
				start = left
			}
			if row == last {
				end = min(end, right+1)
			}
		}
		out = append(out, ansi.Cut(text, start, end))
	}
	return strings.Join(out, "\n")
}

func (a *App) yank() tea.Cmd {
	text := ""
	if a.visualMode != "" {
		text = a.selectedText()
	} else if a.selected >= 0 && a.selected < len(a.Session.Messages) {
		text = a.Session.Messages[a.selected].Content
	}
	if text == "" {
		a.notice = "nothing selected"
		return nil
	}
	linewise := a.visualMode == "visual-line"
	a.visualMode = ""
	return a.copyText(text, linewise)
}

func (a *App) copyText(text string, linewise bool) tea.Cmd {
	a.Prompt.Register = []rune(text)
	a.Prompt.Linewise = linewise
	a.visualMode = ""
	a.notice = "copied to wraith register"
	argv := append([]string(nil), a.Config.Clipboard...)
	if len(argv) == 0 {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		return copiedMsg{Err: cmd.Run()}
	}
}

func (a *App) selectionLine(text string, row int, p palette, lines []rendered) string {
	if a.visualMode == "" {
		return text
	}
	first, left, last, right := a.selectionBounds()
	blockSelected := visualBlockRow(lines, row, first, last)
	if (row < first || row > last) && !blockSelected {
		return text
	}
	width := ansi.StringWidth(text)
	start, end := 0, width
	if a.visualMode == "visual" && !visualBlockRow(lines, row, first, last) {
		if row == first {
			start = left
		}
		if row == last {
			end = min(end, right+1)
		}
	}
	foreground := p.BG
	if foreground == "" {
		foreground = "#101419"
	}
	style := "\x1b[" + color(foreground, false) + ";" + color(p.Second, true) + "m"
	return ansi.Cut(text, 0, start) + style + ansi.Strip(ansi.Cut(text, start, end)) + reset + ansi.Cut(text, end, width)
}

func visualBlockRow(lines []rendered, row, first, last int) bool {
	if row < 0 || row >= len(lines) || lines[row].Block < 0 {
		return false
	}
	block := lines[row]
	for i := max(0, first); i <= last && i < len(lines); i++ {
		if lines[i].Message == block.Message && lines[i].Block == block.Block {
			return true
		}
	}
	return false
}
