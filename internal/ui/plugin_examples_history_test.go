package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
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

// TestHistoryPackageKeepsEarlierPageErrors reports corrupt records across the entire loaded catalog.
func TestHistoryPackageKeepsEarlierPageErrors(t *testing.T) {
	a := exampleFixture(t, "history", `{}`, `wraith.events.on("history.loaded",function() wraith.notify("complete") end)`)
	record := runner.Record{ID: fmt.Sprintf("%024x", 1), Session: a.Session.ID, Directory: a.Directory, Command: "printf good", Status: "completed"}
	if err := a.history().Save(record); err != nil {
		t.Fatal(err)
	}
	for index := range 6 {
		path := filepath.Join(a.history().Root, fmt.Sprintf("%024x.json.gz", index+2))
		if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runScheduledProgram(t, a, a.runCommand("command-history"))
	reader := a.inputReader()
	if reader == nil || !strings.Contains(reader.Buffer.String(), "6 unreadable records") {
		t.Fatalf("earlier page errors missing: %s", a.notice)
	}
}

// TestHistoryPackageRecoversWhenItsBufferIsDeletedDuringQuery resets loading after the reply loses its view.
func TestHistoryPackageRecoversWhenItsBufferIsDeletedDuringQuery(t *testing.T) {
	a := exampleFixture(t, "history", `{}`, `wraith.events.on("history.loaded",function() wraith.notify("complete") end)`)
	_, command := a.Update(a.runCommand("command-history")())
	var pending workspaceQueryMsg
	var collect func(tea.Cmd)
	collect = func(command tea.Cmd) {
		if command == nil {
			return
		}
		switch message := command().(type) {
		case tea.BatchMsg:
			for _, child := range message {
				collect(child)
			}
		case workspaceQueryMsg:
			pending = message
		default:
			a.Update(message)
		}
	}
	collect(command)
	if pending.state == nil {
		t.Fatal("history query did not produce a reply")
	}
	for _, buffer := range a.plugins.Surfaces().Index().Buffers {
		if buffer.Name != "command history" {
			continue
		}
		delta, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "buffer.delete", ID: buffer.ID, Expected: buffer.Revision})
		if err != nil {
			t.Fatal(err)
		}
		a.applySurfaceDelta(delta, true)
	}
	applyPluginEvents(t, a, a.deliverWorkspaceQuery(pending))
	a.notice = ""
	runScheduledProgram(t, a, a.runCommand("command-history"))
	if a.focusedSurfaceView() == nil {
		t.Fatal("history remained stuck after buffer deletion")
	}
}

// TestHistoryPackageBoundsLongCommandPreviews avoids overflowing a view while loading valid large records.
func TestHistoryPackageBoundsLongCommandPreviews(t *testing.T) {
	a := exampleFixture(t, "history", `{}`, `wraith.events.on("history.loaded",function() wraith.notify("complete") end)`)
	for index := range 40 {
		record := runner.Record{ID: fmt.Sprintf("%024x", index+1), Session: a.Session.ID, Directory: a.Directory,
			Command: "printf " + strings.Repeat("界", 11000), Status: "completed"}
		if err := a.history().Save(record); err != nil {
			t.Fatal(err)
		}
	}
	runScheduledProgram(t, a, a.runCommand("command-history"))
	text := a.inputReader().Buffer.String()
	if len(text) > 16<<10 || !strings.Contains(text, "...") || !utf8.ValidString(text) {
		t.Fatal("history retained unbounded or invalid command previews")
	}
}
