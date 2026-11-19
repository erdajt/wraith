package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/fsutil"
	"github.com/erdajt/wraith/internal/terminal"
)

type stagedEdit struct {
	PTY           *terminal.PTY
	Path, Pending string
}

type editedMsg struct {
	Staged         *stagedEdit
	Kind, Language string
	File           *editor.File
	Index          int
	Err            error
}

// edit prepares an editing file and hands terminal control to the configured editor.
func (a *App) edit(kind, language, text string, index int) tea.Cmd {
	if a.busy {
		a.notice = "finish or cancel the current operation before editing"
		return nil
	}
	if a.demo {
		a.notice = "external editing is disabled in demo mode"
		return nil
	}

	var staged *stagedEdit
	if kind == "staged" {
		if index < 0 || index >= len(a.panes) || a.panes[index] == nil {
			a.notice = "staged shell no longer exists"
			return nil
		}
		p := a.panes[index]
		if p.Pending == "" || p.PasteID != "" {
			a.notice = "wait for insertion acknowledgement before editing the staged command"
			return nil
		}
		staged = &stagedEdit{PTY: p.PTY, Path: p.Path, Pending: p.Pending}
	}

	ext := ".md"
	switch kind {
	case "environment":
		ext = ".env"
	case "prompt":
		text = a.Prompt.String()
	case "instructions":
		text = a.Instructions
	case "config":
		candidate := a.Config
		if a.plugins != nil {
			var err error
			candidate, err = config.LoadDirectory(a.Paths.Config)
			if err != nil {
				a.notice = err.Error()
				return nil
			}
		}
		a.configEditBase = &candidate
		b, _ := config.Encode(candidate, "config.yaml")
		text = string(b) + "\n"
		ext = ".yaml"
	case "pin":
		if index < 0 || index >= len(a.Session.Pins) {
			a.notice = "pin no longer exists"
			return nil
		}
		text = a.Session.Pins[index]
	case "response":
		if index < 0 || index >= len(a.Session.Messages) {
			a.notice = "no response selected"
			return nil
		}
		text = a.messageAt(index)
	case "block", "run", "staged":
		ext = editor.Extension(language)
		if kind == "block" || kind == "staged" {
			if err := a.refreshVariables(); err != nil {
				a.notice = err.Error()
				return nil
			}
			if _, err := a.Variables.Validate(text, true); err != nil {
				a.notice = err.Error()
				return nil
			}
			if ext == ".sh" {
				var err error
				text, err = a.Variables.Shell(text)
				if err != nil {
					a.notice = err.Error()
					return nil
				}
			} else {
				text, _ = a.Variables.Expand(text)
			}
		}
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
	a.editorFile = f
	a.editing = true
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editedMsg{Kind: kind, Language: language, File: f, Index: index, Err: err, Staged: staged}
	})
}

// editResponse edits an assistant response or requests a fork before editing a user message.
func (a *App) editResponse() tea.Cmd {
	i := a.selected
	if i < 0 || i >= len(a.Session.Messages) {
		i = len(a.Session.Messages) - 1
		for i >= 0 && a.Session.Messages[i].Role != "assistant" {
			i--
		}
	}

	if i >= 0 && a.Session.Messages[i].Role == "user" {
		return a.confirmFork(i)
	}
	return a.edit("response", "", "", i)
}

// finishEdit validates and applies saved edits, retaining the temporary file when application fails.
func (a *App) finishEdit(m editedMsg) tea.Cmd {
	a.editing = false
	text, saved, err := m.File.Result(m.Err)
	applied := false
	a.editorSucceeded = false
	defer func() {
		a.editorSucceeded = applied
		if applied || (!saved && err == nil) {
			m.File.Remove()
		} else {
			a.notice += " · edit retained at " + m.File.Path
		}
	}()

	if err != nil {
		a.notice = "editor: " + err.Error()
		return nil
	}
	if !saved {
		a.notice = "editor closed without saving"
		return nil
	}

	switch m.Kind {
	case "environment":
		_, err = a.envEdit.Store.Save(a.Directory, a.envEdit.Scope, a.envEdit.Revision, text)
		if err != nil {
			a.notice = err.Error()
			return nil
		}
		if err = a.refreshVariables(); err != nil {
			a.notice = err.Error()
			return nil
		}
		applied = true
		return a.environmentCommand(string(a.envEdit.Scope), true)
	case "prompt":
		a.Prompt.Set(text)
		a.focus = "prompt"
		applied = true
		a.notice = "prompt updated · " + a.Config.Keys["send"] + " sends"
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
		applied = true
		a.notice = "persistent instructions saved"
	case "config":
		c, e := config.Load(m.File.Path)
		if e != nil {
			a.notice = "config unchanged: " + e.Error()
			return nil
		}
		effective := c
		if a.plugins != nil {
			effective, e = a.mergeYamlEdit(c)
			if e != nil {
				a.notice = "config unchanged: " + e.Error()
				return nil
			}
		}
		registry, e := a.buildKeymaps(effective)
		if e != nil {
			a.notice = "config unchanged: " + e.Error()
			return nil
		}
		if err = config.Save(config.File(a.Paths.Config), c); err != nil {
			a.notice = err.Error()
			return nil
		}
		applied = true
		c = effective
		a.configEditBase = nil
		a.Config, a.keymaps, a.keyPending = c, registry, nil
		a.Client = a.providerClient(c)
		a.Session.Model = c.Model
		a.Session.Provider = c.Provider
		a.resizePanes()
		a.persist("settings saved · shell changes apply to new panes")
		return a.probe()
	case "pin":
		if m.Index < 0 || m.Index >= len(a.Session.Pins) || len(text) > 64<<10 {
			a.notice = "pin is missing or exceeds 64 kib"
			return nil
		}

		candidate := a.Session.Clone()
		candidate.Pins[m.Index] = strings.TrimSpace(text)
		if candidate.Pins[m.Index] == "" {
			a.notice = "empty pin rejected; use unpin to remove it"
			return nil
		}

		if err := a.Store.Save(candidate); err != nil {
			a.notice = err.Error()
			return nil
		}
		a.Session = candidate
		applied = true
		return a.memoryCommand("pins", "")
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
		applied = true
		a.Session = candidate
		a.notice = "response edited · original archived"
	case "run":
		if a.proposal == nil {
			a.notice = "command approval expired"
			return nil
		}
		if strings.TrimSpace(text) == "" || strings.ContainsAny(text, "\x00\x1b\r") {
			a.notice = "edited command is empty or contains terminal controls"
			return nil
		}
		command, e := a.proposal.Variables.Shell(text)
		if e != nil {
			a.notice = e.Error()
			return nil
		}
		a.proposal.Command, a.proposal.Resolved, a.proposal.Text = text, command, text
		a.proposal.Source += "-edited"
		a.proposal.policyEdited = true
		applied = true
		return a.runApproved()
	case "block", "staged":
		index := 0
		if m.Kind == "staged" {
			index = m.Index
			if m.Staged == nil || index < 0 || index >= len(a.panes) {
				a.notice = "staged edit has no shell target"
				return nil
			}
			p := a.panes[index]
			if p == nil || p.PTY != m.Staged.PTY || p.Path != m.Staged.Path || p.Pending != m.Staged.Pending || p.PasteID != "" {
				a.notice = "staged command changed while the editor was open"
				return nil
			}
		}
		if m.Kind == "staged" && strings.TrimSpace(text) == "" {
			a.notice = "empty staged command rejected"
			return nil
		}
		if strings.Contains(text, "wraith::") {
			if editor.Extension(m.Language) != ".sh" {
				a.notice = "resolve private references in the source before saving"
				return nil
			}
			if err := a.refreshVariables(); err != nil {
				a.notice = err.Error()
				return nil
			}
			text, err = a.Variables.Shell(text)
			if err != nil {
				a.notice = err.Error()
				return nil
			}
		}

		path, command, e := editor.Stage(filepath.Join(a.Paths.Data, "snippets"), m.Language, text)
		if e != nil {
			a.notice = e.Error()
			return nil
		}

		applied = true
		cmd := a.openTerminal(index)
		if a.panes[index] == nil {
			a.notice = "saved snippet at " + path + "; shell could not start"
			return cmd
		}
		p := a.panes[index]
		if m.Kind != "staged" && (p.Pending != "" || p.PasteID != "") {
			a.notice = "previous command is still staged · new snippet saved at " + path
			return cmd
		}
		p.Path = path
		p.Language = m.Language
		if command == "" {
			a.notice = "saved " + path + " · no interpreter mapped for " + m.Language
		} else {
			p.Pending = command
			return tea.Batch(cmd, a.insertSaved(index))
		}
		return cmd
	}
	return nil
}

// editStaged reopens a staged snippet using its saved language and contents.
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
	return a.edit("staged", p.Language, string(b), index)
}
