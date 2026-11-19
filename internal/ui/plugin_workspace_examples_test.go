package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/provider"
)

// exampleFixture loads the shipped module through the real package lifecycle.
func exampleFixture(t *testing.T, name, options, extra string) *App {
	t.Helper()
	root := os.Getenv("WRAITH_TEST_EXAMPLES")
	if root == "" {
		root = filepath.Join("..", "..", "examples", "plugins")
	}
	path, err := filepath.Abs(filepath.Join(root, name+".lua"))
	if err != nil {
		t.Fatal(err)
	}
	source := `package.preload.proof=assert(loadfile(` + strconv.Quote(path) + `))
 wraith.plugins.load("proof", ` + options + `)
 wraith.commands.create("unloadproof",function() wraith.plugins.unload("proof") end)
 wraith.commands.create("reloadproof",function() wraith.plugins.load("proof", ` + options + `) end)
 ` + extra
	return pluginFixture(t, source)
}

// TestTaskPackageTransformsRequestsAndUnloadsOwnedUI exercises the shipped picker and request hook.
func TestTaskPackageTransformsRequestsAndUnloadsOwnedUI(t *testing.T) {
	a := exampleFixture(t, "tasks", `{}`, `wraith.events.on("tasks.selected",function() wraith.notify("complete") end)`)
	applyPluginEvents(t, a, a.runCommand("task"))
	if a.pluginDialog == nil {
		t.Fatalf("task picker missing: %s", a.notice)
	}
	runScheduledProgram(t, a, a.key(tea.KeyMsg{Type: tea.KeyEnter}))
	data := provider.PromptData{Model: "local", Messages: []provider.PromptMessage{{Role: "user", Content: "inspect"}}}
	client := a.providerClient(a.Config)
	transformed, err := client.Transform(t.Context(), "chat", data)
	if err != nil || len(transformed.Messages) != 2 || transformed.Messages[0].Content != "workspace task: review the changes, identify defects and cite the affected files" {
		t.Fatalf("task prompt = %+v, %v", transformed, err)
	}
	if len(a.plugins.Surfaces().Index().Regions) != 1 {
		t.Fatal("task header missing")
	}
	applyPluginEvents(t, a, a.runCommand("unloadproof"))
	if len(a.plugins.Surfaces().Index().Regions) != 0 || a.providerClient(a.Config).Transform != nil {
		t.Fatal("unloaded task retained UI or request hook")
	}
	applyPluginEvents(t, a, a.runCommand("reloadproof"))
	applyPluginEvents(t, a, a.runCommand("task"))
	if a.pluginDialog == nil {
		t.Fatal("reloaded task command unavailable")
	}
}
