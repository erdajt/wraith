package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

// TestReadmeManagerExample executes the published spec and module through the first lazy command.
func TestReadmeManagerExample(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	var source string
	for _, section := range strings.Split(string(readme), "```lua\n")[1:] {
		block, _, closed := strings.Cut(section, "\n```")
		if closed && strings.Contains(block, `require("wraith.manager").setup`) {
			if source != "" {
				t.Fatal("README has multiple package setup examples; select each explicitly")
			}
			source = block
		}
	}
	if source == "" {
		t.Fatal("README package setup example is missing")
	}
	directory := t.TempDir()
	module, err := os.ReadFile(filepath.Join("..", "..", "docs", "examples", "lua", "plugins", "review.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "plugins"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string][]byte{"init.lua": []byte(source), "plugins/review.lua": module} {
		if err := os.WriteFile(filepath.Join(directory, name), contents, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := OpenWithOptions(t.Context(), directory, config.Default(), nil, managerOptions(t))
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
			t.Fatalf("missing README command %q", name)
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
	first := invoke("review.draft", "session recovery")
	replayed, ready := false, false
	for _, effect := range first.Effects {
		switch effect.Name {
		case "command.review.draft":
			result := invoke("review.draft", effect.Argument)
			if len(result.Effects) != 1 || result.Effects[0].Name != "prompt.set" || result.Effects[0].Argument != "review session recovery for correctness and missing tests" {
				t.Fatalf("README command lost its input: %+v", result)
			}
			replayed = true
		case "event.dispatch":
			delivery := effect.Data.(EventDelivery)
			pending, err := r.SubmitEvent(effect.Argument, delivery.Listeners, map[string]any{"data": delivery.Data}, r.Manifest().Config, "")
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != "review commands loaded" {
				t.Fatalf("README ready observer failed: %+v", result)
			}
			ready = true
		default:
			t.Fatalf("unexpected README activation effect: %+v", effect)
		}
	}
	if !replayed || !ready {
		t.Fatalf("missing lazy handoff: replayed=%v ready=%v", replayed, ready)
	}
}
