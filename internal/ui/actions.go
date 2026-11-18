package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/keymap"
)

type action struct {
	description string
	run         func(string) tea.Cmd
}

// builtinActions exposes workspace operations through one named dispatch table.
func (a *App) builtinActions() map[string]action {
	actions := map[string]action{
		"operation.cancel": {"cancel model work or interrupt the focused shell", func(string) tea.Cmd { return a.cancelInput() }},
		"noop":             {"consume the binding", func(string) tea.Cmd { return nil }},
		"app.quit":         {"save and quit", func(string) tea.Cmd { return a.quit() }},
		"prompt.send":      {"send the draft", func(string) tea.Cmd { return a.submit() }},
		"editor.open": {"edit the focused message or draft", func(string) tea.Cmd {
			if a.focus == "chat" {
				return a.editResponse()
			}
			return a.edit("prompt", "", "", -1)
		}},
		"focus.next": {"switch prompt and conversation focus", func(string) tea.Cmd {
			a.chatCount, a.chatMotionPending, a.chatMotionCount = "", "", ""
			a.chatG, a.chatZ = false, false
			if a.focus == "chat" {
				a.focus = "prompt"
			} else {
				a.focus = "chat"
				a.ensureChatCursor()
			}
			return nil
		}},
		"terminal.overlay": {"toggle the centered shell", func(string) tea.Cmd { return a.toggleTerminal(0) }},
		"terminal.side":    {"toggle the side shell", func(string) tea.Cmd { return a.toggleTerminal(1) }},
	}
	for _, spec := range commandSpecs {
		actions["command."+spec.Name] = action{spec.Description, func(arg string) tea.Cmd {
			return a.runCommand(strings.TrimSpace(spec.Name + " " + arg))
		}}
	}
	for _, specs := range subcommands {
		for _, spec := range specs {
			actions["command."+spec.Name] = action{spec.Description, func(arg string) tea.Cmd {
				return a.runCommand(strings.TrimSpace(spec.Name + " " + arg))
			}}
		}
	}
	for _, spec := range windowActions {
		actions[spec.name] = action{spec.description, func(string) tea.Cmd {
			if a.popup != nil || strings.HasPrefix(a.focus, "terminal") {
				a.notice = "return to the conversation before changing windows"
				return nil
			}
			a.windowAction(strings.ReplaceAll(spec.key, "<lt>", "<"))
			return nil
		}}
	}
	return actions
}

var windowActions = []struct{ name, key, description string }{
	{"window.split_horizontal", "s", "split above and below"},
	{"window.split_vertical", "v", "split side by side"},
	{"window.left", "h", "focus left"}, {"window.down", "j", "focus below"},
	{"window.up", "k", "focus above"}, {"window.right", "l", "focus right"},
	{"window.next", "w", "cycle visible windows"}, {"window.close", "c", "close this window"},
	{"window.only", "o", "keep only this window"}, {"window.equalize", "=", "equalize window sizes"},
	{"window.grow_height", "+", "grow window height"}, {"window.shrink_height", "-", "shrink window height"},
	{"window.grow_width", ">", "grow window width"}, {"window.shrink_width", "<lt>", "shrink window width"},
}

// buildKeymaps validates named actions and installs configured overrides over built-in chords.
func (a *App) buildKeymaps(c config.Config) (*keymap.Registry, error) {
	var bindings []keymap.Binding
	for _, mode := range []string{"chat.normal", "chat.visual"} {
		for _, spec := range windowActions {
			bindings = append(bindings, keymap.Binding{Mode: mode, Keys: "<C-w>" + spec.key, Action: spec.name, Description: spec.description})
		}
	}
	for setting, name := range map[string]string{"send": "prompt.send", "editor": "editor.open", "focus": "focus.next"} {
		for _, mode := range []string{"prompt.insert", "prompt.normal", "prompt.visual", "prompt.operator", "chat.normal", "chat.visual", "chat.operator"} {
			bindings = append(bindings, keymap.Binding{Mode: mode, Keys: keymap.Notation(c.Keys[setting]), Action: name})
		}
	}
	for setting, name := range map[string]string{"terminal_overlay": "terminal.overlay", "terminal_split": "terminal.side", "quit": "app.quit", "cancel": "operation.cancel"} {
		bindings = append(bindings, keymap.Binding{Mode: "*", Keys: keymap.Notation(c.Keys[setting]), Action: name})
	}
	bindings = append(bindings, c.Keymaps...)
	for i := range bindings {
		binding := &bindings[i]
		entry, ok := a.actions[binding.Action]
		if !ok {
			return nil, fmt.Errorf("keymap %q: unknown action %q", binding.Keys, binding.Action)
		}
		if binding.Description == "" {
			binding.Description = entry.description
		}
	}
	return keymap.New(bindings)
}

// runAction dispatches a registered operation without interpreting it as terminal input.
func (a *App) runAction(name, arg string) tea.Cmd {
	entry, ok := a.actions[name]
	if !ok {
		a.notice = "unknown action: " + name
		return nil
	}
	return entry.run(arg)
}
