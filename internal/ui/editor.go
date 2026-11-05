package ui

import (
	"encoding/json"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/ollama"
	"path/filepath"
)

type editedMsg struct {
	Kind, Language string
	File           *editor.File
	Index          int
	Err            error
}

func (a *App) edit(kind, language, text string, index int) tea.Cmd {
	if a.busy {
		a.notice = "finish or cancel the current operation before editing"
		return nil
	}
	if a.demo {
		a.notice = "external editing is disabled in demo mode"
		return nil
	}
	ext := ".md"
	switch kind {
	case "prompt":
		text = a.Prompt.String()
	case "instructions":
		text = a.Instructions
	case "config":
		b, _ := json.MarshalIndent(a.Config, "", "  ")
		text = string(b) + "\n"
		ext = ".json"
	case "response":
		if index < 0 || index >= len(a.Session.Messages) {
			a.notice = "no response selected"
			return nil
		}
		text = a.Session.Messages[index].Content
	case "block":
		ext = editor.Extension(language)
	}
	f, err := editor.Prepare(filepath.Join(a.Paths.Cache, "edits"), ext, text)
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	cmd, err := f.Command(a.Config.Editor)
	if err != nil {
		f.Remove()
		a.notice = err.Error()
		return nil
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editedMsg{kind, language, f, index, err} })
}

func (a *App) editResponse() tea.Cmd {
	i := a.selected
	if i < 0 || i >= len(a.Session.Messages) || a.Session.Messages[i].Role != "assistant" {
		i = len(a.Session.Messages) - 1
		for i >= 0 && a.Session.Messages[i].Role != "assistant" {
			i--
		}
	}
	return a.edit("response", "", "", i)
}

func (a *App) finishEdit(m editedMsg) tea.Cmd {
	defer m.File.Remove()
	text, saved, err := m.File.Result(m.Err)
	if err != nil {
		a.notice = "editor: " + err.Error()
		return nil
	}
	if !saved {
		a.notice = "editor closed without saving"
		return nil
	}
	switch m.Kind {
	case "prompt":
		a.Prompt.Set(text)
		a.focus = "prompt"
		a.notice = "prompt updated · ctrl+s sends"
	case "instructions":
		if len(text) > 64<<10 {
			a.notice = "persistent instructions exceed 64 kib"
			return nil
		}
		if err = fsutil.Bytes(filepath.Join(a.Paths.Config, "instructions.md"), []byte(text)); err != nil {
			a.notice = err.Error()
			return nil
		}
		a.Instructions = text
		a.notice = "persistent instructions saved"
	case "config":
		c, e := config.Load(m.File.Path)
		if e != nil {
			a.notice = "config unchanged: " + e.Error()
			return nil
		}
		if err = config.Save(filepath.Join(a.Paths.Config, "config.json"), c); err != nil {
			a.notice = err.Error()
			return nil
		}
		a.Config = c
		a.Client = ollama.New(c.Host)
		a.Session.Model = c.Model
		a.resizePanes()
		a.persist("settings saved · shell changes apply to new panes")
		return a.probe()
	case "response":
		candidate := a.Session.Clone()
		archive, e := a.Store.Archive(candidate)
		if e != nil {
			a.notice = e.Error()
			return nil
		}
		candidate.Messages[m.Index].Content = text
		candidate.Messages[m.Index].Edited = true
		candidate.Archives = append(candidate.Archives, archive)
		candidate.Usage.InputEstimate = 0
		if err = a.Store.Save(candidate); err != nil {
			a.notice = err.Error()
			return nil
		}
		a.Session = candidate
		a.notice = "response edited · original archived"
	case "block":
		path, command, e := editor.Stage(filepath.Join(a.Paths.Data, "snippets"), m.Language, text)
		if e != nil {
			a.notice = e.Error()
			return nil
		}
		cmd := a.openTerminal(0)
		if a.panes[0] == nil {
			a.notice = "saved snippet at " + path + "; shell could not start"
			return cmd
		}
		p := a.panes[0]
		p.Path = path
		p.Language = m.Language
		if command == "" {
			a.notice = "saved " + path + " · no interpreter mapped for " + m.Language
		} else {
			p.Pending = command
			a.notice = "saved code is staged · enter inserts the command; a second enter executes"
		}
		return cmd
	}
	return nil
}

func (a *App) editStaged(index int) tea.Cmd {
	p := a.panes[index]
	if p == nil || p.Path == "" {
		return nil
	}
	b, err := fsutil.ReadLimit(p.Path, 4<<20)
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	return a.edit("block", p.Language, string(b), -1)
}
