package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestPluginCallbacksReturnOrderedOperationsAndIsolatedSnapshots(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"init.lua": `wraith.plugins.load("workspace", {greeting = "ready"})`,
		"workspace.lua": `return {setup = function(opts)
  wraith.commands.create("arrange", function(ctx)
    local windows = wraith.windows.list()
    windows[1].id = 99
    assert(wraith.windows.list()[1].id == 1)
    assert(wraith.session.get().id == "session")
    wraith.windows.split("vertical")
    wraith.windows.focus(1)
    wraith.prompt.set(wraith.prompt.get() .. " edited")
    wraith.notify(opts.greeting)
  end)
  return function() end
end, teardown = function() end}`,
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	m := r.Manifest()
	snapshot := map[string]any{"session": map[string]any{"id": "session"}, "prompt": "draft", "windows": []any{map[string]any{"id": 1}}}
	result, err := r.Submit(m.Commands[0].Callback, "", snapshot, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := <-result
	if got.Err != nil || len(got.Effects) != 4 || got.Effects[0].Name != "window.split_vertical" || got.Effects[2].Argument != "draft edited" || got.Effects[3].Argument != "ready" {
		t.Fatalf("callback result: %+v", got)
	}
}

func TestNativeInputRejectsInvalidAndOversizedCallbackBatches(t *testing.T) {
	for _, body := range []string{
		`wraith.input.feed("dd<F1>")`,
		`wraith.input.set_mode("replace")`,
		`for i = 1, 9 do wraith.input.feed("abcdefghijklmnop") end`,
	} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			source := `wraith.commands.create("invalid", function() ` + body + ` end)`
			if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := Open(t.Context(), dir, config.Default(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			m := r.Manifest()
			result, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			if got := <-result; got.Err == nil {
				t.Fatal("invalid native input batch accepted")
			}
		})
	}
}
