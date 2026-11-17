package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/variables"
)

func (a *App) variableStore() variables.Store {
	return variables.Store{Root: filepath.Join(a.Paths.Config, "variables"), Conversation: a.Session.ID}
}

func (a *App) refreshVariables() error {
	a.Variables = variables.Snapshot{}
	a.variableInventory = ""
	state, err := a.variableStore().Load(a.Directory, variables.Directory)
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
	scope := variables.Scope(a.Config.Environment.Scope)
	prefix, rest, _ := strings.Cut(arg, " ")
	switch variables.Scope(prefix) {
	case variables.Conversation, variables.Directory, variables.Global:
		scope, arg = variables.Scope(prefix), strings.TrimSpace(rest)
	}
	global := scope == variables.Global
	if global && arg != "" && !confirmed {
		a.envPending = "global " + arg
		a.popup = &menu{Kind: "env-global", Title: "global variables apply in every directory", Items: []item{{"cancel", "cancel", "leave variables unchanged"}, {"continue", "continue", "names and descriptions become available in all chats"}}}
		return nil
	}
	if arg == "edit" || arg == "new" {
		state, err := a.variableStore().Edit(a.Directory, scope)
		if err != nil {
			a.notice = err.Error()
			return nil
		}
		if scope == variables.Conversation {
			if err := a.Store.Save(a.Session); err != nil {
				a.notice = err.Error()
				return nil
			}
		}
		a.envEdit.Store, a.envEdit.Scope, a.envEdit.Revision = a.variableStore(), scope, state.Revision
		a.envScope = scope
		return a.edit("environment", "", state.Text, -1)
	}
	state, err := a.variableStore().Load(a.Directory, scope)
	if err != nil {
		a.notice = err.Error()
		return nil
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
			if scope == variables.Conversation {
				e = a.Store.Save(a.Session)
			}
			if e == nil {
				_, e = a.variableStore().Save(a.Directory, scope, state.Revision, text)
			}
		}
		if e == nil {
			e = a.refreshVariables()
		}
		if e != nil {
			a.notice = e.Error()
			return nil
		}
		notice := "saved private variable " + name
		if operation == "unset" {
			notice = "removed private variable " + name
			if _, err := a.Variables.Validate("wraith::"+name, true); !global && err == nil {
				notice += " · another scoped binding remains active"
			}
		}
		index := 0
		if a.popup != nil && a.popup.Kind == "environment" {
			index = a.popup.Index
		}
		a.environmentCommand(string(scope), true)
		if a.popup != nil && a.popup.Kind == "environment" {
			a.popup.Index = max(0, min(index, len(a.popup.Items)-1))
		}
		a.notice = notice
		return nil
	}
	if arg != "" {
		a.notice = "usage: /env · /env edit · /env set NAME=value · /env unset NAME · /env conversation|directory|global"
		return nil
	}
	var items []item
	for _, entry := range state.Entries {
		items = append(items, item{Label: entry.Name + "=" + entry.Value, Value: entry.Name, Detail: entry.Description})
	}
	label := string(scope)
	a.envScope = scope
	a.popup = &menu{Kind: "environment", Title: "private variables · " + label, Hints: a.Config.Keys["env_new"] + "/" + a.Config.Keys["env_edit"] + " editor · " + a.Config.Keys["env_scope"] + " scope · " + a.Config.Keys["env_remove"] + " remove", Items: items}
	return nil
}

func (a *App) environmentKey(k tea.KeyMsg) (tea.Cmd, bool) {
	prefix := string(a.envScope) + " "
	switch k.String() {
	case a.Config.Keys["env_edit"], a.Config.Keys["env_new"]:
		return a.environmentCommand(prefix+"edit", false), true
	case a.Config.Keys["env_scope"]:
		a.popup = &menu{Kind: "env-scope", Title: "manage variable scope", Items: []item{
			{"conversation", "conversation", "only this saved conversation"},
			{"directory", "directory", "chats in this working directory"},
			{"global", "global", "all conversations and directories"},
		}}
		return nil, true
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
	state, err := a.variableStore().Load(a.Directory, a.envScope)
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
