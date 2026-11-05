package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
	"path/filepath"
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
		return a.approve(proposal{Command: command, Text: b.Text, Language: b.Language, Source: "block"})
	}
	return nil
}
