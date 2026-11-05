package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/editor"
	"strings"
)

func (a *App) messageAt(index int) string {
	if index >= 0 && index < len(a.Session.Messages) {
		return a.Session.Messages[index].Content
	}
	if index == len(a.Session.Messages) && a.busy {
		return a.partial.Content
	}
	return ""
}

func (a *App) editCursorBlock(line rendered) tea.Cmd {
	blocks := editor.Blocks(a.messageAt(line.Message))
	if line.Block < 0 || line.Block >= len(blocks) {
		a.notice = "block is still streaming"
		return nil
	}
	block := blocks[line.Block]
	if block.Language == "wraith-run" {
		request, err := contextual.RequestedCommand("```wraith-run\n" + block.Text + "\n```")
		if err != nil || request == nil {
			a.notice = "invalid command request"
			return nil
		}
		block.Language = "sh"
		block.Text = request.Command
	}
	return a.edit("block", block.Language, strings.TrimRight(block.Text, "\n"), -1)
}
