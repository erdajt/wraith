package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/variables"
)

func (a *App) variableStore() variables.Store {
	return variables.Store{Root: filepath.Join(a.Paths.Config, "variables")}
}

func (a *App) refreshVariables() error {
	state, err := a.variableStore().Load(a.Directory, false)
	if err != nil {
		return err
	}
	inventory, err := state.Snapshot.Instructions()
	if err != nil {
		return err
	}
	a.Variables = state.Snapshot
	a.variableInventory = inventory
	client := *a.Client
	client.Variables = state.Snapshot
	a.Client = &client
	return nil
}

func (a *App) chatInstructions() string {
	return a.Instructions + a.variableInventory
}

func (a *App) environmentCommand(arg string, confirmed bool) tea.Cmd {
	global := arg == "global" || strings.HasPrefix(arg, "global ")
	if global {
		arg = strings.TrimSpace(strings.TrimPrefix(arg, "global"))
	}
	if global && arg != "" && !confirmed {
		a.envPending = "global " + arg
		a.popup = &menu{Kind: "env-global", Title: "global variables apply in every directory", Items: []item{{"cancel", "cancel", "keep directory scope"}, {"continue", "continue", "names and descriptions become available in all chats"}}}
		return nil
	}
	state, err := a.variableStore().Load(a.Directory, global)
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	if arg == "edit" || arg == "new" {
		a.envRevision, a.envGlobal = state.Revision, global
		return a.edit("environment", "", state.Text, -1)
	}
	if strings.HasPrefix(arg, "set ") || strings.HasPrefix(arg, "unset ") {
		operation, input, _ := strings.Cut(arg, " ")
		name, value, description := strings.TrimSpace(input), "", ""
		if operation == "set" {
			entries, e := variables.Parse(input)
			if e != nil || len(entries) != 1 {
				a.notice = "use /env set NAME=value with one valid assignment"
				return nil
			}
			name, value, description = entries[0].Name, entries[0].Value, entries[0].Description
		}
		text, e := variables.Rewrite(state.Text, name, value, operation == "unset")
		if e == nil && description != "" {
			text, e = variables.Rewrite(text, name, "", true)
			text += "\n" + input + "\n"
		}
		if e == nil {
			_, e = a.variableStore().Save(a.Directory, global, state.Revision, text)
		}
		if e == nil {
			e = a.refreshVariables()
		}
		if e != nil {
			a.notice = e.Error()
			return nil
		}
		a.notice = "saved private variable " + name
		return nil
	}
	if arg != "" {
		a.notice = "usage: /env · /env edit · /env set NAME=value · /env unset NAME · /env global"
		return nil
	}
	var items []item
	for _, entry := range state.Entries {
		items = append(items, item{Label: entry.Name + "=" + entry.Value, Value: entry.Name, Detail: entry.Description})
	}
	scope := "directory"
	if global {
		scope = "global"
	}
	a.envGlobal = global
	a.popup = &menu{Kind: "environment", Title: "private variables · " + scope + " · n/e editor · g scope · d remove", Items: items}
	return nil
}

func (a *App) environmentKey(k tea.KeyMsg) (tea.Cmd, bool) {
	prefix := ""
	if a.envGlobal {
		prefix = "global "
	}
	switch k.String() {
	case a.Config.Keys["env_edit"], a.Config.Keys["env_new"]:
		return a.environmentCommand(prefix+"edit", false), true
	case a.Config.Keys["env_scope"]:
		if a.envGlobal {
			return a.environmentCommand("", false), true
		}
		return a.environmentCommand("global edit", false), true
	case a.Config.Keys["env_remove"]:
		if len(a.popup.Items) > 0 {
			name := a.popup.Items[a.popup.Index].Value
			return a.environmentCommand(prefix+"unset "+name, false), true
		}
		return nil, true
	}
	return nil, false
}

func (a *App) variableDetails(name string) {
	state, err := a.variableStore().Load(a.Directory, a.envGlobal)
	if err != nil {
		a.notice = err.Error()
		return
	}
	for _, entry := range state.Entries {
		if entry.Name == name {
			a.showText("environment-detail", "private variable · local value", fmt.Sprintf("%s=%s\n\ndescription shared with assistant:\n%s", entry.Name, entry.Value, state.Snapshot.Redact(entry.Description)))
			return
		}
	}
}
