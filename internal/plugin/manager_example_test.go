package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

// TestDocumentedPackageConfiguration runs the published files through normal imports and lazy activation.
func TestDocumentedPackageConfiguration(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join("..", "..", "docs", "examples", "lua", "packages")
	if err := os.CopyFS(directory, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	r, err := Open(t.Context(), directory, config.Default(), nil, config.Paths{Data: t.TempDir(), Cache: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	invoke := func(name, argument string) Result {
		t.Helper()
		var callback string
		for _, command := range r.Manifest().Commands {
			if command.Name == name {
				callback = command.Callback
			}
		}
		if callback == "" {
			t.Fatalf("missing documented command %q", name)
		}
		pending, err := r.Submit(callback, "", map[string]any{"argument": argument}, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if result.Manifest != nil {
			if err := r.Acknowledge(result.Revision, true); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	first := invoke("review", "error handling")
	if len(first.Effects) != 1 || first.Effects[0].Name != "command.review" || first.Effects[0].Argument != "error handling" {
		t.Fatalf("lazy activation lost the command argument: %+v", first)
	}
	result := invoke("review", "error handling")
	want := "review the current changes and explain any correctness issues\n\nfocus: error handling"
	if len(result.Effects) != 1 || result.Effects[0].Name != "prompt.set" || result.Effects[0].Argument != want {
		t.Fatalf("documented package did not prepare the draft: %+v", result)
	}

	invoke("packages.unload", "review")
	for _, command := range r.Manifest().Commands {
		if command.Name == "review" {
			t.Fatal("package unload retained its command")
		}
	}
	invoke("packages.load", "review")
	result = invoke("review", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "review the current changes and explain any correctness issues" {
		t.Fatalf("explicit reload did not restore the documented command: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(directory, "wraith-lock.json")); !os.IsNotExist(err) {
		t.Fatalf("local example unexpectedly wrote a package lock: %v", err)
	}
}
