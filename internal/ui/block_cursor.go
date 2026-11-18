package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/editor"
)

// messageAt returns user-facing content for a saved or currently streaming message.
func (a *App) messageAt(index int) string {
	if index >= 0 && index < len(a.Session.Messages) {
		return contextual.DisplayContent(a.Session.Messages[index].Content)
	}
	if index == len(a.Session.Messages) && a.busy {
		return contextual.DisplayContent(a.partial.Content)
	}
	return ""
}

// editCursorBlock opens the complete code block under the conversation cursor in the editor.
func (a *App) editCursorBlock(line rendered) tea.Cmd {
	blocks := editor.Blocks(a.messageAt(line.Message))
	if line.Block < 0 || line.Block >= len(blocks) {
		a.notice = "block is still streaming"
		return nil
	}
	block := blocks[line.Block]
	return a.edit("block", block.Language, strings.TrimRight(block.Text, "\n"), -1)
}
