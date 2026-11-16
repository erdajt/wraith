package provider

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestEveryProviderRequestRedactsConversationAndPreservesCaller(t *testing.T) {
	c := config.Default()
	c.Provider = "openai"
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "private.example") || !strings.Contains(string(body), "wraith::TARGET") {
			t.Fatalf("outbound privacy boundary failed")
		}
		return response("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"), nil
	})
	request := ollama.Request{Model: "model", Messages: []session.Message{{Role: "system", Content: "archive private.example"}, {Role: "user", Content: "check private.example"}}}
	if _, err := client.Chat(t.Context(), request, nil); err != nil {
		t.Fatal(err)
	}
	if request.Messages[0].Content != "archive private.example" {
		t.Fatal("caller request was mutated")
	}
}

func TestPrivateRequestScrubsAliasedEnvironmentAndRejectsAuthCollision(t *testing.T) {
	t.Setenv("WRAITH_PRIVATE_ALIAS", "url=private.example")
	t.Setenv("WRAITH_PUBLIC_TEST", "retained")
	c := config.Default()
	c.Provider = "codex"
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	dir := t.TempDir()
	script := filepath.Join(dir, "record")
	source := "#!/bin/sh\nenv > \"$1/env\"\ncat > \"$1/input\"\nprintf '%s\\n' '{\"type\":\"turn.completed\"}'\n"
	if err := os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	client.Profile.Command = []string{script, dir}
	if _, err := client.Chat(t.Context(), ollama.Request{Messages: []session.Message{{Role: "user", Content: "private.example"}}}, nil); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"env", "input"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "private.example") {
			t.Fatalf("private value reached child %s", file)
		}
		if file == "env" && !strings.Contains(string(data), "WRAITH_PUBLIC_TEST=retained") {
			t.Fatal("ordinary environment lost")
		}
	}
	t.Setenv("OPENAI_API_KEY", "private.example")
	if _, err := client.Chat(t.Context(), ollama.Request{}, nil); err == nil {
		t.Fatal("auth collision was transmitted")
	}
}
