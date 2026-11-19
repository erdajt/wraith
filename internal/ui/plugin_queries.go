package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
)

type workspaceQueryState struct {
	runtime              *plugin.Runtime
	query                plugin.WorkspaceQuery
	cancel               context.CancelFunc
	context              context.Context
	cancelled, delivered bool
}

type workspaceQueryMsg struct {
	state *workspaceQueryState
	data  map[string]any
}

// startWorkspaceQuery captures immutable inputs and starts host reads outside the UI loop.
func (a *App) startWorkspaceQuery(runtime *plugin.Runtime, query plugin.WorkspaceQuery) tea.Cmd {
	if runtime != a.plugins || !workspaceQueryLive(runtime, query) {
		runtime.DiscardScheduled([]string{query.ID})
		return nil
	}
	if a.pluginQueries == nil {
		a.pluginQueries = map[string]*workspaceQueryState{}
		a.workspaceScans = &workspaceScanStore{}
	}
	if _, exists := a.pluginQueries[query.ID]; exists {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	state := &workspaceQueryState{runtime: runtime, query: query, cancel: cancel, context: ctx}
	if len(a.pluginQueries) >= 16 {
		cancel()
		return a.deliverWorkspaceQuery(workspaceQueryMsg{state: state, data: workspaceQueryResult(query, nil, errors.New("workspace query capacity reached"))})
	}
	a.pluginQueries[query.ID] = state
	input := workspaceQueryInput{
		query: query, runtime: runtime, scans: a.workspaceScans,
		store: a.Store, history: runner.History{Root: filepath.Join(a.Paths.Data, "commands")},
		session: session.Session{ID: a.Session.ID, Directory: a.Session.Directory}, directory: a.Directory,
	}
	if strings.HasPrefix(query.Kind, "sessions.") {
		input.session = a.Session.Clone()
	}
	if a.jobs != nil && strings.HasPrefix(query.Kind, "jobs.") {
		input.jobs = a.jobs.Snapshot()
	}
	if query.Kind == "environment.list" {
		inventory, err := a.Variables.Inventory()
		if err != nil {
			cancel()
			return a.deliverWorkspaceQuery(workspaceQueryMsg{state: state, data: workspaceQueryResult(query, nil, err)})
		}
		var items any
		if err := json.Unmarshal([]byte(inventory), &items); err != nil {
			cancel()
			return a.deliverWorkspaceQuery(workspaceQueryMsg{state: state, data: workspaceQueryResult(query, nil, err)})
		}
		input.data = map[string]any{"items": items, "directory": a.Directory, "session": a.Session.ID, "scope": a.Config.Environment.Scope}
	}
	if query.Kind == "terminals.list" {
		items := []any{}
		for index, pane := range a.panes {
			if pane == nil || pane.Screen == nil {
				continue
			}
			items = append(items, map[string]any{
				"id":    fmt.Sprintf("terminal:%d:%d:%d", index, pane.Started.UnixNano(), pane.Restarts),
				"index": index, "generation": pane.Restarts, "visible": pane.Visible,
				"running": pane.PTY != nil && pane.Err == "", "width": pane.Screen.Width, "height": pane.Screen.Height, "error": pane.Err,
			})
		}
		input.data = map[string]any{"items": items}
	}
	return func() tea.Msg {
		result, err := input.run(ctx)
		if err == nil {
			err = ctx.Err()
		}
		return workspaceQueryMsg{state: state, data: workspaceQueryResult(query, result, err)}
	}
}

// workspaceQueryResult limits detached callback payloads independently of source read budgets.
func workspaceQueryResult(query plugin.WorkspaceQuery, result any, err error) map[string]any {
	data := map[string]any{"id": query.ID, "kind": query.Kind, "ok": err == nil}
	if err == nil {
		encoded, encodeErr := json.Marshal(result)
		if encodeErr != nil {
			err = encodeErr
		} else if len(encoded) > 256<<10 {
			err = errors.New("workspace query result exceeds 256 kib; request a smaller page")
		} else {
			var detached any
			if decodeErr := json.Unmarshal(encoded, &detached); decodeErr != nil {
				err = decodeErr
			} else {
				data["result"] = detached
			}
		}
	}
	if err != nil {
		code := "io"
		switch {
		case errors.Is(err, context.Canceled):
			code = "cancelled"
		case errors.Is(err, context.DeadlineExceeded):
			code = "deadline"
		case errors.Is(err, os.ErrNotExist):
			code = "not_found"
		case strings.Contains(err.Error(), "wrong_directory"):
			code = "wrong_directory"
		case strings.Contains(err.Error(), "stale") || strings.Contains(err.Error(), "changed; restart"):
			code = "stale_revision"
		case strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "exceeds") || strings.Contains(err.Error(), "capacity"):
			code = "limit"
		}
		data["ok"] = false
		data["error"] = map[string]any{"code": code, "message": workspacePreview(err.Error(), 2048)}
	}
	return data
}

// deliverWorkspaceQuery preserves a completion through queue pressure and consumes its callback once.
func (a *App) deliverWorkspaceQuery(message workspaceQueryMsg) tea.Cmd {
	state := message.state
	if state.delivered {
		return nil
	}
	if state.cancelled {
		message.data = workspaceQueryResult(state.query, nil, context.Canceled)
	}
	if state.runtime != a.plugins || !workspaceQueryLive(state.runtime, state.query) {
		state.cancel()
		state.runtime.DiscardScheduled([]string{state.query.ID})
		if a.pluginQueries[state.query.ID] == state {
			delete(a.pluginQueries, state.query.ID)
		}
		return nil
	}
	channel, err := state.runtime.Submit(state.query.ID, "", a.pluginSnapshot("", "", message.data), a.Config)
	if errors.Is(err, plugin.ErrQueueFull) {
		return tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg { return message })
	}
	state.delivered = true
	state.cancel()
	if a.pluginQueries[state.query.ID] == state {
		delete(a.pluginQueries, state.query.ID)
	}
	if err != nil {
		state.runtime.DiscardScheduled([]string{state.query.ID})
		a.notice = "lua workspace query: " + err.Error()
		return nil
	}
	pending := pluginResultMsg{Runtime: state.runtime, Owner: a.keyOwner(), Draft: a.Prompt.String(), Digest: configDigest(a.Config), Target: a.pluginTarget(), Keys: keymapDigest(a.Config), LeaseOwner: state.query.Owner, Lease: state.query.Lease}
	return func() tea.Msg {
		select {
		case pending.Result = <-channel:
			return pending
		case <-a.done:
			return nil
		}
	}
}

// cancelWorkspaceQuery closes owned reads and their unconsumed scan continuations.
func (a *App) cancelWorkspaceQuery(runtime *plugin.Runtime, owner, lease, id string) {
	if a.workspaceScans != nil {
		a.workspaceScans.cancel(runtime, owner, lease, id)
	}
	if state := a.pluginQueries[id]; state != nil && state.runtime == runtime && state.query.Owner == owner && state.query.Lease == lease {
		state.cancelled = true
		state.cancel()
	}
}

// reconcileWorkspaceQueries discards dead owners without invoking Lua cleanup from workers.
func (a *App) reconcileWorkspaceQueries(closeAll bool) {
	for id, state := range a.pluginQueries {
		if closeAll || state.runtime != a.plugins || !workspaceQueryLive(state.runtime, state.query) {
			state.cancel()
			state.runtime.DiscardScheduled([]string{id})
			delete(a.pluginQueries, id)
		}
	}
	if a.workspaceScans != nil {
		a.workspaceScans.reconcile(a.plugins, closeAll)
	}
}

// workspaceQueryLive keeps provider-owned queries inside their invocation lifetime.
func workspaceQueryLive(runtime *plugin.Runtime, query plugin.WorkspaceQuery) bool {
	return runtime.Surfaces().OwnerValid(query.Owner, query.Lease) && (query.Invocation == "" || runtime.InvocationValid(query.Invocation))
}
