package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
)

func (a *App) blockActions(block editor.Block) tea.Cmd {
	a.selectedBlock = block
	a.popup = &menu{Kind: "block-actions", Title: "code block · " + block.Language, Items: []item{{"edit", "edit", "save to stage in scratch shell"}, {"run", "run", "review and capture output"}, {"view", "view", "show the full block"}}}
	return nil
}

func (a *App) blockAction(action string) tea.Cmd {
	b := a.selectedBlock
	switch action {
	case "edit":
		return a.edit("block", b.Language, b.Text, -1)
	case "view":
		a.showText("block-preview", "code block · esc returns", b.Text)
	case "run":
		if a.demo {
			a.notice = "command execution is disabled in demo mode"
			return nil
		}
		path, command, err := editor.Stage(filepath.Join(a.Paths.Data, "snippets"), b.Language, b.Text)
		if err != nil {
			a.notice = err.Error()
			return nil
		}
		if command == "" {
			a.notice = "saved " + path + "; no interpreter mapped"
			return nil
		}
		a.proposal = &proposal{Command: command, Text: b.Text, Language: b.Language, Source: "block"}
		return a.runApproved()
	}
	return nil
}

func (a *App) selectedBlocks() tea.Cmd {
	index := a.selected
	if index < 0 || index >= len(a.Session.Messages) || a.Session.Messages[index].Role != "assistant" {
		a.notice = "select an assistant response containing code"
		return nil
	}
	blocks := editor.Blocks(a.messageAt(index))
	if len(blocks) == 1 {
		return a.blockActions(blocks[0])
	}
	if len(blocks) == 0 {
		a.notice = "selected response has no complete code blocks"
		return nil
	}
	items := []item{}
	for i, b := range blocks {
		first, _, _ := strings.Cut(b.Text, "\n")
		items = append(items, item{Label: b.Language + " · " + first, Value: fmt.Sprintf("%d:%d", index, i)})
	}
	a.popup = &menu{Kind: "blocks", Title: "response blocks · choose edit or run", Items: items}
	return nil
}
