package ui

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/session"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func fixture(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	c := config.Default()
	c.Shell = []string{"/bin/sh", "-i"}
	a, err := New(c, config.Paths{Config: root + "/config", Data: root + "/data", Cache: root + "/cache"}, session.New("test-model"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}
func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestModelAliasesDiscoverAndSwitchInstalledModels(t *testing.T) {
	for _, alias := range []string{"model", "models"} {
		t.Run(alias, func(t *testing.T) {
			a := fixture(t)
			a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || r.URL.Path != "/api/tags" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				return response(`{"models":[{"name":"zeta","size":9000000000},{"name":"alpha","size":1000000000}]}`), nil
			})
			cmd := a.runCommand(alias)
			if cmd == nil {
				t.Fatal("missing model request")
			}
			a.Update(cmd())
			if a.popup.Kind != "models" || a.popup.Items[0].Value != "alpha" {
				t.Fatal("installed models not sorted")
			}
			a.Config.Thinking = "high"
			a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
			saved, err := a.Store.Load(a.Session.ID)
			if err != nil || saved.Model != "alpha" || a.Config.Thinking != "auto" {
				t.Fatalf("switch not persisted: %+v, %v", saved, err)
			}
		})
	}
}

func TestDiscardOverflowDoesNotSendStaleDraft(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("retain this draft")
	a.pendingPrompt = a.Prompt.String()
	a.popup = &menu{Kind: "overflow"}
	a.menuKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.pendingPrompt != "" || a.Prompt.String() != "retain this draft" {
		t.Fatal("closing overflow changed the draft or left a pending send")
	}
}

func TestInvalidSavedConfigRetainsEditedFile(t *testing.T) {
	a := fixture(t)
	f, err := editor.Prepare(a.Paths.Cache, ".json", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.Path, []byte(`{"context":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "config", File: f})
	if _, err = os.Stat(f.Path); err != nil {
		t.Fatal("invalid edits were deleted")
	}
	if a.Config.Context != 8192 || !strings.Contains(a.notice, f.Path) {
		t.Fatal("invalid config replaced settings or recovery path missing")
	}
}

func TestDemoCommandsCannotWriteUserData(t *testing.T) {
	a := fixture(t)
	a.demo = true
	for _, cmd := range []string{"remember change", "new", "export " + filepath.Join(a.Paths.Data, "export.md"), "restore x", "models"} {
		a.runCommand(cmd)
	}
	if _, err := os.Stat(a.Paths.Data); !os.IsNotExist(err) {
		t.Fatal("demo created data")
	}
	if a.Instructions != "" {
		t.Fatal("demo changed persistent instructions")
	}
}

func TestSpaceEventsSurvivePromptAndCommandEditing(t *testing.T) {
	a := fixture(t)
	for _, k := range []tea.KeyMsg{key("host"), {Type: tea.KeySpace, Runes: []rune{' '}}, key("name")} {
		a.key(k)
	}
	if a.Prompt.String() != "host name" {
		t.Fatalf("prompt lost space: %q", a.Prompt.String())
	}
	a.commandMode = true
	for _, k := range []tea.KeyMsg{key("set"), {Type: tea.KeySpace, Runes: []rune{' '}}, key("context")} {
		a.key(k)
	}
	if a.command != "set context" {
		t.Fatalf("command lost space: %q", a.command)
	}
}
