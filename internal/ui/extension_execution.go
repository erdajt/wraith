package ui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// runExtension rechecks the frozen definition before starting a tracked background invocation.
func (a *App) runExtension(p proposal) tea.Cmd {
	if a.busy || a.demo || p.Extension == nil {
		return nil
	}
	call := p.Extension
	if p.launch.Session != a.Session.ID || p.launch.Generation != a.generation || a.Config.Execution.Mode != "agent" {
		a.proposal, a.popup = nil, nil
		a.notice = "tool approval belongs to an earlier conversation or execution mode"
		return nil
	}
	if a.plugins == nil {
		a.notice = "extension runtime is unavailable"
		return nil
	}
	definitions := a.plugins.Manifest().Tools
	index := slices.IndexFunc(definitions, func(d plugin.ToolDefinition) bool { return d.Name == call.Definition.Name })
	if a.plugins != call.Runtime || index < 0 || !reflect.DeepEqual(definitions[index], call.Definition) {
		a.proposal, a.popup = nil, nil
		cmd := a.proposeExtension(call.Definition.Name, call.Arguments)
		if a.proposal != nil {
			a.notice = "tool definition changed · review a new approval"
		}
		return cmd
	}
	if err := call.Schema.Validate(call.Arguments); err != nil {
		a.notice = "tool arguments: " + err.Error()
		return nil
	}
	arguments := slices.Clone(call.Arguments)
	definition := call.Definition
	record := p.launch
	record.Command = definition.Name + " " + string(arguments)
	record.OutputHidden, record.ShareOutput = p.HideOutput, !p.HideOutput
	binding := plugin.InvocationBinding{Kind: "tool", Name: definition.Name, Method: "run", Owner: definition.Owner, Lease: definition.Lease, Callback: definition.Callback, Revision: definition.Revision}
	a.jobs.SetLimit(a.Config.Execution.MaxJobs)
	err := a.jobs.StartTask(context.Background(), record, p.timeout, func(ctx context.Context, output io.Writer) error {
		request, err := json.Marshal(map[string]any{"arguments": json.RawMessage(arguments), "job_id": record.ID, "directory": record.Directory})
		if err != nil {
			return err
		}
		invocation, err := call.Runtime.BeginInvocation(ctx, plugin.InvocationRequest{Binding: binding, Data: request, Timeout: p.timeout})
		if err != nil {
			return err
		}
		defer invocation.Close()
		for {
			event, err := invocation.Next(ctx)
			if err != nil {
				return err
			}
			if event.Error != "" {
				return errors.New(event.Error)
			}
			if _, err = output.Write(append(event.Data, '\n')); err != nil {
				return err
			}
			if event.Final {
				return nil
			}
		}
	})
	if err != nil {
		a.showExtensionApproval(p)
		a.notice = "tool launch: " + err.Error()
		return nil
	}
	a.proposal = nil
	return a.acceptJob(record, true)
}
