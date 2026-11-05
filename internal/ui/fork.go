package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
	"time"
)

func (a *App) confirmFork(index int) tea.Cmd {
	a.forkIndex = index
	a.popup = &menu{Kind: "fork", Title: "fork conversation before this message?", Items: []item{
		{"yes · edit this prompt", "yes", "newer messages are removed from the fork; original stays saved"},
		{"no · keep this conversation", "no", "return without changes"},
	}}
	return nil
}

func (a *App) forkPrompt() tea.Cmd {
	index := a.forkIndex
	if a.demo || a.busy || index < 0 || index >= len(a.Session.Messages) || a.Session.Messages[index].Role != "user" {
		a.notice = "message is no longer available for editing"
		return nil
	}
	original := a.Session.Clone()
	if err := a.Store.Save(original); err != nil {
		a.notice = err.Error()
		return nil
	}
	branch := original.Clone()
	branch.ID = session.ID()
	branch.Title = "fork · " + original.Title
	branch.Created = time.Now().UTC()
	branch.Messages = branch.Messages[:index]
	branch.SharedRuns = nil
	branch.Usage = session.Usage{}
	branch.Declines = 0
	if err := a.Store.Save(branch); err != nil {
		a.notice = err.Error()
		return nil
	}
	a.Session = branch
	a.Prompt.Set(original.Messages[index].Content)
	a.Prompt.Mode = "normal"
	a.Prompt.Pos = 0
	a.focus = "prompt"
	a.follow = true
	a.selected = max(0, index-1)
	a.chatCursor = -1
	a.visualMode = ""
	a.agentSteps = 0
	a.proposal = nil
	a.notice = "fork opened · edit the draft, then :w or ctrl+s sends · original is in /resume"
	return nil
}
