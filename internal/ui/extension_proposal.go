package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/modeltool"
	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

type extensionProposal struct {
	Runtime    *plugin.Runtime
	Definition plugin.ToolDefinition
	Schema     *modeltool.Schema
	Arguments  json.RawMessage
}

// proposeExtension freezes a tool definition, validated arguments and private substitutions for approval.
func (a *App) proposeExtension(name string, arguments json.RawMessage) tea.Cmd {
	if a.plugins == nil || a.busy || a.demo || a.Config.Execution.Mode != "agent" {
		a.notice = "extension tools require an idle agent conversation"
		return nil
	}
	if a.agentSteps >= a.Config.Execution.MaxSteps {
		a.agentStopped = true
		a.notice = "tool step limit reached · send a prompt to continue"
		return nil
	}
	definitions := a.plugins.Manifest().Tools
	index := slices.IndexFunc(definitions, func(d plugin.ToolDefinition) bool { return d.Name == name })
	if index < 0 {
		a.notice = "extension tool is unavailable: " + name
		return nil
	}
	definition := definitions[index]
	schema, err := modeltool.Compile(definition.Parameters)
	if err == nil {
		err = schema.Validate(arguments)
	}
	if err != nil {
		a.notice = "extension arguments: " + err.Error()
		return nil
	}
	if err = a.refreshVariables(); err != nil {
		a.notice = err.Error()
		return nil
	}
	resolved, err := resolveToolArguments(arguments, a.Variables, schema)
	if err != nil {
		a.notice = "extension arguments: " + err.Error()
		return nil
	}
	timeout, err := time.ParseDuration(a.Config.Execution.CommandTimeout)
	if err != nil {
		a.notice = err.Error()
		return nil
	}
	if a.generation == "" {
		a.generation = session.ID()
	}
	p := proposal{
		Command: name, Source: "extension:" + name, Reason: definition.Description, Continue: true, Background: true, prepared: true,
		Variables: a.Variables, timeout: timeout,
		Extension: &extensionProposal{Runtime: a.plugins, Definition: definition, Schema: schema, Arguments: resolved},
		launch:    runner.Record{ID: session.ID(), Session: a.Session.ID, Generation: a.generation, Directory: a.Directory, Source: "extension:" + name, Background: true, Agent: true, AgentRequested: true, ShareOutput: true, CaptureLimit: a.Config.Execution.CaptureBytes, TailLimit: a.Config.Execution.TailBytes},
	}
	return a.showExtensionApproval(p)
}

// resolveToolArguments expands string values without turning private data into JSON syntax or field names.
func resolveToolArguments(arguments json.RawMessage, snapshot variables.Snapshot, schema *modeltool.Schema) (json.RawMessage, error) {
	if err := schema.Validate(arguments); err != nil {
		return nil, err
	}
	encoded, err := modeltool.TransformArguments(arguments, func(value string) (string, error) {
		if _, err := snapshot.Validate(value, true); err != nil {
			return "", err
		}
		resolved, _ := snapshot.Expand(value)
		return resolved, nil
	})
	if err == nil {
		err = schema.Validate(encoded)
	}
	return encoded, err
}

// showExtensionApproval presents concrete arguments without displaying the internal action protocol.
func (a *App) showExtensionApproval(p proposal) tea.Cmd {
	var pretty bytes.Buffer
	_ = json.Indent(&pretty, p.Extension.Arguments, "", "  ")
	p.Resolved = p.Extension.Definition.Name + "\n" + pretty.String()
	a.proposal = &p
	a.popup = &menu{Kind: "approval", Title: "approve extension tool", Items: []item{
		{"run", "run", "execute and share bounded output"},
		{"run · hide output from agent", "run-hidden", "execute without sharing output"},
		{"edit arguments", "edit", "save valid json to execute once"},
		{"view full request", "view", "inspect the tool and complete arguments"},
		{"reject", "cancel", "stop this agent turn"},
	}}
	return nil
}

// extensionAction handles native approval decisions for a frozen local tool call.
func (a *App) extensionAction(action string) tea.Cmd {
	p := a.proposal
	switch action {
	case "run", "run-hidden":
		p.HideOutput = action == "run-hidden"
		return a.runExtension(*p)
	case "edit":
		var pretty bytes.Buffer
		_ = json.Indent(&pretty, p.Extension.Arguments, "", "  ")
		return a.edit("extension-tool", "json", pretty.String(), -1)
	case "view":
		a.showText("command-preview", "extension request", fmt.Sprintf("%s\n\nreason: %s\nowner: %s\nworking directory: %s\ntimeout: %s", p.Resolved, p.Reason, p.Extension.Definition.Owner, p.launch.Directory, p.timeout))
		a.popup.Hints = "esc returns to approval"
	case "cancel":
		a.declineCommand()
	}
	return nil
}
