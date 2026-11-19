package ui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

// TestWorkspaceJobResyncRetainsForgottenIdentity keeps completion discoverable after native delivery.
func TestWorkspaceJobResyncRetainsForgottenIdentity(t *testing.T) {
	a := pluginFixture(t, ``)
	a.jobs = runner.NewManager(runner.History{Root: t.TempDir()}, 1)
	t.Cleanup(a.jobs.Close)
	record := runner.Record{ID: session.ID(), Session: a.Session.ID, Directory: a.Directory, Shell: "/bin/sh", Command: "printf private-output", OutputHidden: true}
	if err := a.jobs.Start(t.Context(), record, time.Second); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		jobs := a.jobs.Snapshot()
		if len(jobs) == 1 && jobs[0].Record.Status == "completed" {
			break
		}
		select {
		case <-a.jobs.Updates:
		case <-deadline.C:
			t.Fatal("native command did not finish")
		}
	}
	a.observeWorkspaceServices()
	a.jobs.Forget(record.ID)
	a.observeWorkspaceServices()
	pending := a.workspaceEvents.pending["jobs_changed"]
	if pending == nil || len(pending["items"].([]any)) != 0 || len(a.workspaceEvents.removed) != 1 || a.workspaceEvents.removed[0] != record.ID {
		t.Fatalf("completion resync = %#v, removed %v", pending, a.workspaceEvents.removed)
	}
	data, err := json.Marshal(pending)
	if err != nil || strings.Contains(string(data), "private-output") {
		t.Fatalf("resync exposed output: %s, %v", data, err)
	}
}

// TestWorkspaceVariableObservationsReactToDescriptionAndScopeChanges keeps metadata views current.
func TestWorkspaceVariableObservationsReactToDescriptionAndScopeChanges(t *testing.T) {
	a := pluginFixture(t, ``)
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example", Description: "first"}}, nil)
	a.observeWorkspaceServices()
	first := a.workspaceEvents.pending["environment_changed"]["sequence"].(uint64)
	delete(a.workspaceEvents.pending, "environment_changed")
	a.observeWorkspaceServices()
	if a.workspaceEvents.pending["environment_changed"] != nil {
		t.Fatal("unchanged inventory emitted another observation")
	}
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example", Description: "second"}}, nil)
	a.Config.Environment.Scope = "conversation"
	a.observeWorkspaceServices()
	pending := a.workspaceEvents.pending["environment_changed"]
	if pending["sequence"].(uint64) <= first || pending["scope"] != "conversation" {
		t.Fatalf("metadata change not observed: %#v", pending)
	}
	data, err := json.Marshal(pending)
	if err != nil || strings.Contains(string(data), "private.example") || strings.Contains(string(data), "second") {
		t.Fatalf("observation exceeded names-only payload: %s, %v", data, err)
	}
}
