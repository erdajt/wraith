package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
)

func TestRemoteStartupDoesNotLoadAnOllamaModel(t *testing.T) {
	c := config.Default()
	c.Provider = "openai"
	client := New(c)
	calls := 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/v1/models" || r.Method != "GET" {
			t.Error("remote startup tried local loading")
		}
		return response(`{"data":[{"id":"remote-model"}]}`), nil
	})
	if err := client.EnsureServer(context.Background(), true, "ignored"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Show(context.Background(), "remote-model"); err != nil {
		t.Fatal(err)
	}
	if err := client.Load(context.Background(), "remote-model", "2m", ollama.Options{}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("startup calls %d", calls)
	}
}
