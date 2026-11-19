package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFilesPackageReadsRealGitStateAndReleasesSurfaces exercises managed processes and owned buffers.
func TestFilesPackageReadsRealGitStateAndReleasesSurfaces(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	directory := t.TempDir()
	command := exec.Command("git", "init", "--quiet", directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", output, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "changed.go"), []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a := exampleFixture(t, "files", `{directory=`+strconv.Quote(directory)+`}`, `wraith.events.on("files.refreshed",function() wraith.notify("complete") end)`)
	runScheduledProgram(t, a, tea.Batch(a.runCommand("changed-files"), a.startPluginResources()))
	found := false
	for _, item := range a.plugins.Surfaces().Index().Buffers {
		buffer, err := a.plugins.Surfaces().Buffer(item.ID)
		if err != nil {
			t.Fatal(err)
		}
		found = found || strings.Contains(strings.Join(buffer.Lines, "\n"), "?? changed.go")
	}
	if !found {
		t.Fatalf("real git output missing: %s", a.notice)
	}
	applyPluginEvents(t, a, a.runAction("window.close", ""))
	a.notice = ""
	runScheduledProgram(t, a, tea.Batch(a.runCommand("changed-files"), a.watchPluginResources(a.plugins)))
	if a.focusedSurfaceView() == nil {
		t.Fatal("closed changed-files view did not reopen")
	}
	applyPluginEvents(t, a, a.runCommand("unloadproof"))
	for _, item := range a.plugins.Surfaces().Index().Buffers {
		if item.Name == "changed files" {
			t.Fatal("unloaded package retained changed-files buffer")
		}
	}
}
