package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestScopePackageChecksApprovedEditsAndUnloadsPolicy uses the shipped file loader and native approval flow.
func TestScopePackageChecksApprovedEditsAndUnloadsPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scope.json")
	a := exampleFixture(t, "scope", `{path=`+strconv.Quote(path)+`}`, `wraith.events.on("scope.loaded",function(ctx) assert(ctx.data.ready); wraith.notify("complete") end)`)
	data, err := json.Marshal(map[string]any{"directory": a.Directory, "commands": []string{"printf allowed"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	runScheduledProgram(t, a, tea.Batch(a.callPlugin("", "ready", ""), a.startPluginResources()))
	a.Update(a.approve(proposal{Command: "printf allowed"})())
	if a.popup == nil || a.popup.Kind != "approval" {
		t.Fatalf("allowed command missing approval: %s", a.notice)
	}
	a.proposal.Command, a.proposal.Resolved, a.proposal.policyEdited = "printf denied", "printf denied", true
	_, next := a.Update(a.runApproved()())
	if next != nil {
		a.Update(next())
	}
	if len(a.jobs.Snapshot()) != 0 || !strings.Contains(a.notice, "exact allowlist") {
		t.Fatalf("edited command bypassed scope: %s", a.notice)
	}
	applyPluginEvents(t, a, a.runCommand("unloadproof"))
	if len(a.pluginManifest.Hooks) != 0 {
		t.Fatal("unloaded scope retained native command policy")
	}
}

// TestScopePackageRejectsTruncatedPolicy refuses valid JSON prefixes beyond the source budget.
func TestScopePackageRejectsTruncatedPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scope.json")
	a := exampleFixture(t, "scope", `{path=`+strconv.Quote(path)+`}`, `wraith.events.on("scope.loaded",function(ctx) assert(not ctx.data.ready); wraith.notify("complete") end)`)
	data := `{"directory":"/tmp","commands":["printf allowed"]}` + strings.Repeat(" ", 65536)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	runScheduledProgram(t, a, tea.Batch(a.callPlugin("", "ready", ""), a.startPluginResources()))
	a.Update(a.approve(proposal{Command: "printf allowed"})())
	if len(a.jobs.Snapshot()) != 0 || a.popup != nil && a.popup.Kind == "approval" || !strings.Contains(a.notice, "65536") {
		t.Fatalf("truncated policy authorized command: %s", a.notice)
	}
}
