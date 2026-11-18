package ui

import (
	"encoding/json"
	"github.com/erdajt/wraith/internal/ollama"
	"net/http"
	"reflect"
	"testing"
)

func TestStartupLoadsChosenModelBeforeAnyChat(t *testing.T) {
	a := fixture(t)
	var paths []string
	a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)

		switch r.URL.Path {
		case "/api/tags":
			return response(`{"models":[]}`), nil
		case "/api/show":
			return response(`{"capabilities":["thinking"]}`), nil
		case "/api/generate":
			return response(`{"done":true}`), nil
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
			return nil, nil
		}
	})
	cmd := a.probe()
	a.Prompt.Set("keep this draft")

	if a.submit() != nil || len(a.Session.Messages) != 0 {
		t.Fatal("submitted while loading")
	}

	a.Update(cmd())

	if a.loading || !a.connected || a.Prompt.String() != "keep this draft" {
		t.Fatal("load state or draft wrong")
	}

	if !reflect.DeepEqual(paths, []string{"/api/tags", "/api/show", "/api/generate"}) {
		t.Fatal(paths)
	}
}

func TestStaleLoadCannotReplaceNewModelState(t *testing.T) {
	a := fixture(t)
	a.probe()
	serial := a.loadSerial
	a.Session.Model = "second"
	a.probe()
	a.Session.Model = "test-model"
	a.probe()
	a.Update(infoMsg{Serial: serial, Model: "test-model"})

	if !a.loading || a.connected {
		t.Fatal("stale load accepted after switching back")
	}
}

func TestPreloadUsesModelCapacity(t *testing.T) {
	for _, override := range []int{0, 1000000} {
		a := fixture(t)
		a.Config = a.Config.WithModelContext(a.Session.Model, override)
		var loaded int
		a.Client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/tags":
				return response(`{"models":[]}`), nil
			case "/api/show":
				return response(`{"model_info":{"test.context_length":4096}}`), nil
			case "/api/generate":
				var req struct {
					Options ollama.Options `json:"options"`
				}

				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatal(err)
				}
				loaded = req.Options.Context
				return response(`{"done":true}`), nil
			default:
				t.Fatalf("unexpected request %s", r.URL.Path)
				return nil, nil
			}
		})
		a.Update(a.probe()())
		want := 4096
		if override > 0 {
			want = override
		}

		if loaded != want || a.contextLimit() != want {
			t.Fatalf("loaded %d, displayed %d, want %d", loaded, a.contextLimit(), want)
		}
	}
}
