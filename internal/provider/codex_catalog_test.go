package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/ollama"
)

func writeCodexCache(t *testing.T, data string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	if err := os.WriteFile(filepath.Join(dir, "models_cache.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCodexCatalogUsesVisibleModelsAndNativeMetadata(t *testing.T) {
	writeCodexCache(t, `{"models":[
		{"slug":"first","visibility":"list","context_window":500000,"default_reasoning_level":"low","supported_reasoning_levels":[{"effort":"low"},{"effort":"ultra"}]},
		{"slug":"hidden","visibility":"hide"},
		{"slug":"second","visibility":"list","context_window":272000},
		{"slug":"first","visibility":"list"},
		{"slug":""}
	]}`)
	models, err := codexCache()
	if err != nil || len(models) != 2 || models[0].Name != "first" || models[1].Name != "second" {
		t.Fatalf("catalog: %+v, %v", models, err)
	}
	info := codexInfo("first", ollama.Info{})
	if info.ContextWindow != 500000 || info.Thinking.Default != "low" {
		t.Fatalf("metadata: %+v", info)
	}
	if _, err := info.Think("ultra"); err != nil {
		t.Fatal(err)
	}
	info.Thinking.Values = []any{"custom"}
	if got := codexInfo("first", info); len(got.Thinking.Values) != 1 || got.Thinking.Values[0] != "custom" {
		t.Fatal("explicit thinking configuration was overwritten")
	}
	if got := codexInfo("unknown", ollama.Info{}); got.ContextWindow != 0 {
		t.Fatal("unknown model inherited another model's capacity")
	}
}

func TestCodexCatalogRejectsMissingOrMalformedMetadata(t *testing.T) {
	for _, data := range []string{`{`, `{"models":[]}`, `{"models":[{"slug":"hidden","visibility":"hide"}]}`} {
		t.Run(data, func(t *testing.T) {
			writeCodexCache(t, data)
			if _, err := codexCache(); err == nil {
				t.Fatal("unusable catalog was accepted")
			}
		})
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	if _, err := codexCache(); err == nil {
		t.Fatal("missing catalog was accepted")
	}
}
