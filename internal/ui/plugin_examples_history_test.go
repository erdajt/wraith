package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/runner"
)

// TestHistoryPackagePagesNativeRecordsAndReadsHiddenLocalOutput uses shipped buffer mappings and trusted queries.
func TestHistoryPackagePagesNativeRecordsAndReadsHiddenLocalOutput(t *testing.T) {
	a := exampleFixture(t, "history", `{max_records=2}`, `wraith.events.on("history.loaded",function() wraith.notify("complete") end)
 wraith.events.on("history.output",function() wraith.notify("complete") end)`)
	for index := range 8 {
		record := runner.Record{ID: fmt.Sprintf("%024x", index+1), Session: a.Session.ID, Directory: a.Directory, Command: fmt.Sprintf("printf job%d", index), Status: "completed", Started: time.Now(), Output: "private local output\n", OutputHidden: true}
		if err := a.history().Save(record); err != nil {
			t.Fatal(err)
		}
	}
	runScheduledProgram(t, a, a.runCommand("command-history"))
	if len(a.workspaceScans.entries) != 0 {
		t.Fatal("history limit retained scan cursor")
	}
	if len(a.Session.Messages) != 0 {
		t.Fatal("history read shared output with model")
	}
	applyPluginEvents(t, a, a.runAction("window.close", ""))
	a.notice = ""
	runScheduledProgram(t, a, a.runCommand("command-history"))
	reader := a.inputReader()
	if reader == nil {
		t.Fatalf("history pane not focused: %s", a.notice)
	}
	a.key(key("g"))
	a.key(key("g"))
	a.key(key("j"))
	a.notice = ""
	runScheduledProgram(t, a, a.key(tea.KeyMsg{Type: tea.KeyEnter}))
	found := false
	for _, item := range a.plugins.Surfaces().Index().Buffers {
		buffer, err := a.plugins.Surfaces().Buffer(item.ID)
		if err != nil {
			t.Fatal(err)
		}
		found = found || buffer.Name == "command output" && strings.Contains(strings.Join(buffer.Lines, "\n"), "private local output")
	}
	if !found || len(a.Session.Messages) != 0 {
		t.Fatal("local history output missing or automatically shared")
	}
	applyPluginEvents(t, a, a.runCommand("unloadproof"))
	for _, item := range a.plugins.Surfaces().Index().Buffers {
		if item.Name == "command history" || item.Name == "command output" {
			t.Fatal("history package retained owned views")
		}
	}
}
