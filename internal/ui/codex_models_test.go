package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
)

func TestCodexModelPickerSwitchesAndPersistsModelCapabilities(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	cache := `{"models":[
		{"slug":"alpha","context_window":500000,"supported_reasoning_levels":[{"effort":"ultra"}]},
		{"slug":"beta","context_window":272000}
	]}`

	if err := os.WriteFile(filepath.Join(dir, "models_cache.json"), []byte(cache), 0600); err != nil {
		t.Fatal(err)
	}

	for _, navigation := range []tea.KeyMsg{key("j"), {Type: tea.KeyDown}} {
		a := fixture(t)
		p := a.Config.Providers["codex"]
		p.Command = []string{"sh"}
		a.Config.Providers["codex"] = p
		a.activateProvider("codex", "default")
		a.Update(a.modelCommand("models", "")())

		if a.popup == nil || len(a.popup.Items) != 2 || a.popup.Items[0].Value != "alpha" {
			t.Fatalf("picker has no native models: %s", a.notice)
		}

		a.key(navigation)
		load := a.key(tea.KeyMsg{Type: tea.KeyEnter})
		if load == nil || a.Session.Model != "beta" {
			t.Fatal("navigation did not select beta")
		}

		a.Update(load())

		if !a.connected || a.contextLimit() != 272000 {
			t.Fatal("selection did not load native metadata")
		}

		a.Update(a.modelCommand("model", "alpha")())

		if a.contextLimit() != 500000 {
			t.Fatal("switching model kept the previous capacity")
		}

		a.set("thinking", "ultra")

		if a.Config.Thinking != "ultra" {
			t.Fatalf("advertised reasoning could not be selected: %s", a.notice)
		}

		a.set("thinking", "unsupported")

		if a.Config.Thinking != "ultra" || !strings.Contains(a.notice, "does not advertise") {
			t.Fatal("unadvertised reasoning was accepted")
		}

		c, err := config.LoadDirectory(a.Paths.Config)
		if err != nil || c.Providers["codex"].Model != "alpha" || c.Thinking != "ultra" {
			t.Fatalf("selection was not persisted: %v", err)
		}

		s, err := a.Store.Load(a.Session.ID)
		if err != nil || s.Model != "alpha" || s.Provider != "codex" {
			t.Fatalf("session lost the selection: %v", err)
		}
	}
}
