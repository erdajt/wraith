package ui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
)

// TestWorkspaceMessagePages retains explicit revisions and UTF-8 byte boundaries beyond snapshots.
func TestWorkspaceMessagePages(t *testing.T) {
	current := session.Session{ID: "0123456789abcdef01234567", Directory: t.TempDir()}
	for index := range 90 {
		current.Messages = append(current.Messages, session.Message{Role: "assistant", Content: "界 " + strings.Repeat("x", index)})
	}
	input := workspaceQueryInput{session: current, directory: current.Directory, query: plugin.WorkspaceQuery{
		Kind: "sessions.messages", Options: plugin.WorkspaceOptions{From: 65, Limit: 10},
	}}
	result, err := input.run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	page := result.(map[string]any)
	if len(page["items"].([]any)) != 10 || page["next"] != 75 || page["complete"] != false {
		t.Fatalf("message page = %#v", page)
	}
	input.query = plugin.WorkspaceQuery{Kind: "sessions.message", Options: plugin.WorkspaceOptions{Index: 65, Revision: page["revision"].(string), Limit: 4}}
	result, err = input.run(t.Context())
	if err != nil || result.(map[string]any)["text"] != "界 " || result.(map[string]any)["next"] != int64(4) {
		t.Fatalf("utf-8 page = %#v, %v", result, err)
	}
	input.query.Options.From = 1
	if _, err := input.run(t.Context()); err == nil {
		t.Fatal("partial utf-8 offset accepted")
	}
	input.query.Options.From = 0
	input.session.Messages = append(append([]session.Message{}, current.Messages...), session.Message{Role: "user", Content: "changed"})
	if _, err := input.run(t.Context()); err == nil || !strings.Contains(err.Error(), "stale_revision") {
		t.Fatalf("stale message page = %v", err)
	}
}

// TestWorkspaceStoredQueries checks saved-session directory guards and trusted hidden output reads.
func TestWorkspaceStoredQueries(t *testing.T) {
	root, directory := t.TempDir(), t.TempDir()
	input := workspaceQueryInput{
		store: store.Store{Root: root}, history: runner.History{Root: filepath.Join(root, "commands")},
		session: session.Session{ID: "0123456789abcdef01234567", Directory: directory}, directory: directory,
	}
	other := session.Session{Version: 1, ID: "1123456789abcdef01234567", Directory: t.TempDir()}
	if err := input.store.Save(other); err != nil {
		t.Fatal(err)
	}
	input.query = plugin.WorkspaceQuery{Kind: "sessions.get", Options: plugin.WorkspaceOptions{ID: other.ID}}
	if _, err := input.run(t.Context()); err == nil || !strings.Contains(err.Error(), "wrong_directory") {
		t.Fatalf("foreign directory query = %v", err)
	}
	input.query.Options.Directory = other.Directory
	if _, err := input.run(t.Context()); err != nil {
		t.Fatalf("explicit trusted directory query: %v", err)
	}

	record := runner.Record{ID: "2123456789abcdef01234567", Session: input.session.ID, Directory: directory,
		Command: "printf private", Output: "secret.example\n", OutputHidden: true, Finished: time.Now()}
	if err := input.history.Save(record); err != nil {
		t.Fatal(err)
	}
	input.query = plugin.WorkspaceQuery{Kind: "jobs.output", Options: plugin.WorkspaceOptions{ID: record.ID}}
	result, err := input.run(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	page := result.(map[string]any)["page"].(session.OutputPage)
	if len(page.Lines) != 1 || page.Lines[0].Text != "secret.example" {
		t.Fatalf("hidden local output = %#v", page)
	}
	stored, err := input.history.Load(record.ID)
	if err != nil || stored.ShareOutput || !stored.OutputHidden {
		t.Fatalf("inspection changed native sharing: %#v, %v", stored, err)
	}
}

// TestWorkspaceQueryResponseBounds preserves structured failure without leaking oversized result data.
func TestWorkspaceQueryResponseBounds(t *testing.T) {
	query := plugin.WorkspaceQuery{ID: "query", Kind: "jobs.output"}
	result := workspaceQueryResult(query, map[string]any{"body": strings.Repeat("x", 256<<10)}, nil)
	if result["ok"] != false || result["result"] != nil || result["error"].(map[string]any)["code"] != "limit" {
		t.Fatalf("oversized result = %#v", result)
	}
	result = workspaceQueryResult(query, nil, context.Canceled)
	if result["error"].(map[string]any)["code"] != "cancelled" {
		t.Fatalf("cancelled result = %#v", result)
	}
}
