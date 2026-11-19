package ui

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type workspaceEventState struct {
	runtime      *plugin.Runtime
	fingerprints map[string][32]byte
	pending      map[string]map[string]any
	jobIDs       []string
	removed      []string
	sequence     uint64
}

// observeWorkspaceServices records bounded resync state before native updates can forget completed jobs.
func (a *App) observeWorkspaceServices() {
	state := &a.workspaceEvents
	if state.runtime != a.plugins || state.pending == nil {
		*state = workspaceEventState{runtime: a.plugins, fingerprints: map[string][32]byte{}, pending: map[string]map[string]any{}}
	}
	if a.plugins == nil {
		return
	}
	jobs, ids := []any{}, []string{}
	if a.jobs != nil {
		for _, job := range a.jobs.Snapshot() {
			record := job.Record
			ids = append(ids, record.ID)
			jobs = append(jobs, map[string]any{
				"id": record.ID, "session": record.Session, "status": record.Status,
				"observed_bytes": record.Observed, "truncated": record.Truncated,
				"share_output": record.ShareOutput, "output_hidden": record.OutputHidden,
			})
		}
	}
	slices.Sort(ids)
	slices.SortFunc(jobs, func(left, right any) int {
		return strings.Compare(left.(map[string]any)["id"].(string), right.(map[string]any)["id"].(string))
	})
	for _, id := range state.jobIDs {
		if !slices.Contains(ids, id) && !slices.Contains(state.removed, id) && len(state.removed) < 128 {
			state.removed = append(state.removed, id)
		}
	}
	state.jobIDs = ids
	terminals := []any{}
	for index, pane := range a.panes {
		if pane == nil || pane.Screen == nil {
			continue
		}
		terminals = append(terminals, map[string]any{
			"id":      fmt.Sprintf("terminal:%d:%d:%d", index, pane.Started.UnixNano(), pane.Restarts),
			"visible": pane.Visible, "running": pane.PTY != nil && pane.Err == "",
			"width": pane.Screen.Width, "height": pane.Screen.Height,
		})
	}
	inventory, err := a.Variables.Inventory()
	variables := []any{}
	if err == nil {
		var entries []struct{ Name string }
		if json.Unmarshal([]byte(inventory), &entries) == nil {
			for _, entry := range entries {
				variables = append(variables, entry.Name)
			}
		}
	}
	groups := map[string]any{"jobs_changed": jobs, "terminals_changed": terminals, "environment_changed": variables}
	for _, event := range []string{"jobs_changed", "terminals_changed", "environment_changed"} {
		items := groups[event]
		encoded, _ := json.Marshal(map[string]any{"items": items, "session": a.Session.ID, "directory": a.Directory})
		if event == "environment_changed" {
			encoded = append(encoded, []byte(inventory+a.Config.Environment.Scope)...)
		}
		fingerprint := sha256.Sum256(encoded)
		if state.fingerprints[event] == fingerprint {
			continue
		}
		state.fingerprints[event] = fingerprint
		state.sequence++
		state.pending[event] = map[string]any{
			"sequence": state.sequence, "items": items, "session": a.Session.ID,
			"directory": a.Directory, "resync": true,
		}
		if event == "environment_changed" {
			state.pending[event]["scope"] = a.Config.Environment.Scope
		}
	}
}

// flushWorkspaceEvents retries coalesced observations without retaining private command output.
func (a *App) flushWorkspaceEvents() tea.Cmd {
	state := &a.workspaceEvents
	if state.runtime != a.plugins || a.plugins == nil {
		return nil
	}
	commands := []tea.Cmd{}
	for _, event := range []string{"jobs_changed", "terminals_changed", "environment_changed"} {
		data := state.pending[event]
		if data == nil {
			continue
		}
		if event == "jobs_changed" {
			data["removed"] = slices.Clone(state.removed)
		}
		command, err := a.queuePluginData("", event, "", data)
		if errors.Is(err, plugin.ErrQueueFull) {
			continue
		}
		delete(state.pending, event)
		if event == "jobs_changed" {
			state.removed = nil
		}
		if err != nil {
			a.notice = "lua workspace event: " + err.Error()
		} else {
			commands = append(commands, command)
		}
	}
	return tea.Batch(commands...)
}
