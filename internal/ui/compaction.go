package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/store"
)

func (a *App) startCompact() tea.Cmd {
	if a.loading {
		a.notice = "wait for model loading before compacting"
		return nil
	}
	if a.busy {
		a.notice = "an operation is already running"
		return nil
	}
	if a.demo {
		a.notice = "compaction is disabled in demo mode"
		return nil
	}
	s := a.Session.Clone()
	c := a.Config
	c.Model = s.Model
	instructions := a.Instructions
	client := a.Client
	think, thinkingErr := a.Info.Think("off")
	if thinkingErr != nil {
		think, _ = a.Info.Think("low")
	}
	if err := (store.Store{Root: filepath.Join(a.Paths.Cache, "compaction")}).Save(s); err != nil {
		a.notice = "could not save recovery cache: " + err.Error()
		return nil
	}
	archive, err := a.Store.Archive(s)
	if err != nil {
		a.notice = "could not archive context: " + err.Error()
		return nil
	}
	timeout, _ := time.ParseDuration(c.Timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	a.cancel = cancel
	a.busy = true
	a.compacting = true
	a.phase = "archiving context"
	go func() {
		defer cancel()
		result, e := contextual.Compact(ctx, client, s, c, instructions, think, func(phase string) { a.post(progressMsg(phase)) })
		a.post(compactMsg{result, archive, e})
	}()
	return a.next()
}

func (a *App) finishCompact(m compactMsg) tea.Cmd {
	a.busy = false
	a.compacting = false
	a.cancel = nil
	if m.Err != nil {
		a.pendingPrompt = ""
		a.pendingAgent = false
		a.notice = "compaction stopped: " + m.Err.Error()
		return nil
	}
	m.Session.Archives = append(m.Session.Archives, m.Archive)
	if err := a.Store.Save(m.Session); err != nil {
		a.pendingPrompt = ""
		a.pendingAgent = false
		a.notice = "could not commit compacted context: " + err.Error()
		return nil
	}
	a.Session = m.Session
	a.resetChatNavigation()
	a.thinkingExpanded = map[string]bool{}
	a.follow = true
	a.selected = max(0, len(a.Session.Messages)-1)
	a.notice = "compacted · original context archived · pins and prior facts retained"
	if a.pendingPrompt != "" {
		pending := a.pendingPrompt
		a.pendingPrompt = ""
		if strings.TrimSpace(a.Prompt.String()) == pending {
			return a.submit()
		}
		a.notice += " · draft changed during compaction; review before sending"
	}
	if a.pendingAgent {
		a.pendingAgent = false
		return a.startChat()
	}
	if a.proposal != nil {
		return a.approve(*a.proposal)
	}
	return nil
}
func (a *App) declineCompact() tea.Cmd {
	a.popup = nil
	a.Session.Declines++
	a.persist(fmt.Sprintf("compaction declined %d/3", a.Session.Declines))
	if a.Session.Declines >= 3 {
		return a.startCompact()
	}
	if a.proposal != nil {
		return a.approve(*a.proposal)
	}
	return nil
}
