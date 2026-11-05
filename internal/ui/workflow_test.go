package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/store"
)

func drain(t *testing.T, a *App) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for a.busy {
		select {
		case msg := <-a.events:
			a.Update(msg)
		case <-deadline:
			t.Fatal("worker did not finish")
		}
	}
}

func TestStreamingPersistsInstructionsAndInterruptedReply(t *testing.T) {
	a := fixture(t)
	a.Instructions = "prefer reproducible checks"
	a.Session.Pins = []string{"lab only"}
	a.Config.Thinking = "off"
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var req ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Think != false || !strings.Contains(req.Messages[0].Content, a.Instructions) || !strings.Contains(req.Messages[0].Content, "lab only") {
			t.Error("request lost native thinking or context")
		}
		return response("{\"message\":{\"content\":\"partial evidence\"}}\n"), nil
	})
	a.Prompt.Set("inspect this trace")
	a.submit()
	drain(t, a)
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Messages) != 2 || !saved.Messages[1].Interrupted || saved.Messages[1].Content != "partial evidence" {
		t.Fatalf("partial reply not recoverable: %+v", saved.Messages)
	}
}

func TestCancellationRetainsSubmittedPrompt(t *testing.T) {
	a := fixture(t)
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	a.Prompt.Set("retain this request")
	a.submit()
	a.runCommand("cancel")
	drain(t, a)
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || len(saved.Messages) != 1 || saved.Messages[0].Content != "retain this request" {
		t.Fatalf("cancel lost prompt: %+v, %v", saved, err)
	}
}

func TestThirdDeclineCompactsAndArchivesOriginal(t *testing.T) {
	a := fixture(t)
	a.Session.Pins = []string{"scope is lab.internal"}
	for range 10 {
		a.Session.Messages = append(a.Session.Messages, session.Message{Role: "user", Content: strings.Repeat("observed evidence in lab.internal; preserve the output. ", 20)}, session.Message{Role: "assistant", Content: strings.Repeat("the suggested check has not yet been executed. ", 20)})
	}
	original := a.Session.Clone()
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var req ollama.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if len(req.Messages) != 2 || req.Messages[0].Content == contextual.System {
			t.Error("compactor received chat instructions")
		}
		content := `{"summary":"lab review in progress","facts":["user observed evidence in lab.internal","suggested checks remain unexecuted"]}`
		if req.Messages[0].Content == contextual.AuditPrompt {
			content = `{"missing":["preserve the observed output"]}`
		}
		b, _ := json.Marshal(ollama.Response{Done: true, Message: session.Message{Content: content}})
		return response(string(b)), nil
	})
	for i := 1; i <= 3; i++ {
		a.declineCompact()
		if i < 3 && a.busy {
			t.Fatal("compacted before third decline")
		}
	}
	drain(t, a)
	if len(a.Session.Messages) != 4 || a.Session.Declines != 0 || len(a.Session.Archives) != 1 {
		t.Fatalf("compaction failed: %s", a.notice)
	}
	archived, err := a.Store.ReadArchive(a.Session.Archives[0])
	if err != nil || len(archived.Messages) != len(original.Messages) {
		t.Fatal("original archive lost")
	}
	cached, err := (store.Store{Root: filepath.Join(a.Paths.Cache, "compaction")}).Load(original.ID)
	if err != nil || len(cached.Messages) != len(original.Messages) {
		t.Fatal("recovery cache lost")
	}
	a.restore(a.Session.Archives[0])
	if a.Session.ID == original.ID || len(a.Session.Messages) != 20 {
		t.Fatal("archive was not forked intact")
	}
}

func TestFailedCompactionDoesNotReplaceActiveContext(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "user", Content: "exact evidence"}}
	original := a.Session.Clone()
	a.pendingPrompt = "draft"
	a.finishCompact(compactMsg{Err: context.Canceled})
	if a.Session.Messages[0] != original.Messages[0] || a.pendingPrompt != "" {
		t.Fatal("cancelled compaction mutated active context")
	}
	candidate := a.Session.Clone()
	candidate.Summary = "replacement"
	if err := os.MkdirAll(a.Paths.Data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Paths.Data, "sessions"), []byte("block writes"), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishCompact(compactMsg{Session: candidate, Archive: "original"})
	if a.Session.Summary != "" {
		t.Fatal("failed disk write committed in memory")
	}
}

func TestSavedBlockStagesWithoutExecutionAndPanesPersist(t *testing.T) {
	a := fixture(t)
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	f, err := editor.Prepare(a.Paths.Cache, ".sh", "original")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.Path, []byte("touch "+editor.Quote(marker)), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "block", Language: "sh", File: f})
	p := a.panes[0]
	if p == nil || p.Pending == "" {
		t.Fatalf("block not staged: %s", a.notice)
	}
	a.toggleTerminal(0)
	a.toggleTerminal(0)
	if a.panes[0].PTY != p.PTY {
		t.Fatal("hiding pane replaced shell")
	}
	a.terminalKey(tea.KeyMsg{Type: tea.KeyEnter})
	if p.Pending != "" {
		t.Fatal("command not inserted")
	}
	if _, err = os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("insertion executed code without shell enter")
	}
	a.toggleTerminal(1)
	if a.panes[1] == nil || a.panes[1].PTY == p.PTY {
		t.Fatal("side pane does not have an independent shell")
	}
}
