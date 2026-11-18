package ui

import (
	"errors"
	"github.com/erdajt/wraith/internal/store"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestVariableRepairReplacesPartialReplyWithoutApprovingIt(t *testing.T) {
	a := fixture(t)
	a.environmentCommand("set TARGET=private.example", false)
	a.popup = nil
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		return response("{\"message\":{\"content\":\"checking wraith::MISSING \"}}\n{\"done\":true}\n"), nil
	})
	a.Prompt.Set("inspect the target")
	a.submit()
	drain(t, a)

	if a.proposal != nil || len(a.Session.Messages) != 2 || !a.Session.Messages[1].Interrupted || !strings.Contains(a.notice, "3 retries") {
		t.Fatalf("invalid retry became actionable: %s %+v", a.notice, a.Session.Messages)
	}

	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || saved.Messages[1].Content != "checking wraith::MISSING " {
		t.Fatalf("failed answer was lost: %v", err)
	}

	paths, err := filepath.Glob(filepath.Join(a.Paths.Cache, "repairs", "archives", "*.json.gz"))
	if err != nil || len(paths) != 3 {
		t.Fatalf("rejected attempts missing: %v %v", paths, err)
	}

	for _, path := range paths {
		cached, err := (store.Store{Root: filepath.Join(a.Paths.Cache, "repairs")}).ReadArchive(filepath.Base(path))
		if err != nil || len(cached.Messages) != 1 || !cached.Messages[0].Interrupted || !strings.Contains(cached.Messages[0].Content, "wraith::MISSING") {
			t.Fatalf("invalid attempt cache: %+v %v", cached, err)
		}
	}

}

func TestSupersededStreamCannotAppendOrCompleteCurrentTurn(t *testing.T) {
	a := fixture(t)
	a.streamSerial = 2
	a.busy = true
	a.partial = session.Message{Role: "assistant", Content: "current"}
	a.receive(streamMsg{Serial: 1, Final: true, Chunk: ollama.Response{Message: session.Message{Content: "stale"}}})

	if !a.busy || a.partial.Content != "current" || len(a.Session.Messages) != 0 {
		t.Fatal("stale callback changed active generation")
	}

	a.receive(streamMsg{Serial: 2, Retry: 1, Problem: "unknown variable MISSING"})

	if a.partial.Content != "" || !strings.Contains(a.phase, "retrying 1/3") {
		t.Fatal("retry not visible")
	}

	a.receive(streamMsg{Serial: 2, Final: true, Err: errors.New("repair exhausted"), Chunk: ollama.Response{Message: session.Message{Role: "assistant", Content: "last rejected"}}})

	if len(a.Session.Messages) != 1 || !a.Session.Messages[0].Interrupted {
		t.Fatal("failed attempt not retained")
	}
}
