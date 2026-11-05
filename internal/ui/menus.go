package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
)

func (a *App) menuKey(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	m := a.popup
	if a.settingHelp(k) {
		return nil
	}
	if a.menuSearch(k) {
		return nil
	}
	if m.Kind == "pins" && !m.Searching {
		if cmd, ok := a.pinKey(k); ok {
			return cmd
		}
	}
	if m.Kind == "leader" && !m.Searching && m.Query == "" {
		if cmd, ok := a.leaderKey(k); ok {
			return cmd
		}
	}
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		if len(m.Count) < 6 {
			m.Count += key
		}
		return nil
	}
	count, _ := strconv.Atoi(m.Count)
	count = max(1, count)
	m.Count = ""
	switch key {
	case "esc", "q":
		if m.Kind == "setting-help" {
			a.popup = a.helpReturn
			a.helpReturn = nil
			return nil
		}
		if m.Kind == "block-preview" {
			return a.blockActions(a.selectedBlock)
		}
		if m.Kind == "command-preview" && a.proposal != nil {
			return a.approve(*a.proposal)
		}
		if m.Kind == "approval" {
			a.declineCommand()
		}
		if m.Kind == "overflow" {
			a.pendingPrompt = ""
			a.pendingAgent = false
		}
		if m.Kind == "compact" {
			return a.declineCompact()
		}
		a.popup = nil
		return nil
	case "j", "down":
		m.Index = min(len(m.Items)-1, m.Index+count)
	case "k", "up":
		m.Index = max(0, m.Index-count)
	case "g", "home":
		m.Index = 0
	case "G", "end":
		m.Index = max(0, len(m.Items)-1)
	case "ctrl+d", "pgdown":
		m.Index = min(len(m.Items)-1, m.Index+8*count)
	case "ctrl+u", "pgup":
		m.Index = max(0, m.Index-8*count)
	case "e":
		if m.Kind == "settings" {
			a.popup = nil
			return a.edit("config", "", "", -1)
		}
	case "y":
		if m.Kind == "compact" || m.Kind == "overflow" {
			a.popup = nil
			return a.startCompact()
		}
	case "n":
		if m.Kind == "compact" {
			return a.declineCompact()
		}
		if m.Kind == "overflow" {
			a.pendingPrompt = ""
			a.pendingAgent = false
			a.popup = nil
			return nil
		}
	case "enter":
		if m.Index < 0 || m.Index >= len(m.Items) {
			a.popup = nil
			return nil
		}
		choice := m.Items[m.Index]
		a.popup = nil
		switch m.Kind {
		case "leader":
			return a.runCommand(choice.Value)
		case "history":
			record, err := a.history().Load(choice.Value)
			if err != nil {
				a.notice = err.Error()
				return nil
			}
			a.showRun(record, record.Output)
		case "block-actions":
			return a.blockAction(choice.Value)
		case "approval":
			return a.approvalAction(choice.Value)
		case "models":
			return a.set("model", choice.Value)
		case "theme":
			return a.set("theme", choice.Value)
		case "thinking":
			return a.set("thinking", choice.Value)
		case "value":
			key, value, _ := strings.Cut(choice.Value, "=")
			return a.set(key, value)
		case "settings":
			return a.settingMenu(choice.Value)
		case "compact":
			if choice.Value == "yes" {
				return a.startCompact()
			}
			return a.declineCompact()
		case "overflow":
			if choice.Value == "yes" {
				return a.startCompact()
			}
			a.pendingPrompt = ""
			a.pendingAgent = false
		case "sessions":
			return a.resume(choice.Value)
		case "archives":
			return a.restore(choice.Value)
		case "blocks":
			var mi, bi int
			_, _ = fmt.Sscanf(choice.Value, "%d:%d", &mi, &bi)
			blocks := editor.Blocks(a.Session.Messages[mi].Content)
			return a.blockActions(blocks[bi])
		}
	}
	return nil
}
