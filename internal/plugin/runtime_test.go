package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
)

func TestRuntimeLoadsLocalModulesAndCloses(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plugins"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{"init.lua": `assert(require("plugins.answer") == 42); assert(io == nil and os == nil and debug == nil and dofile == nil and loadfile == nil and print == nil)`, "plugins/answer.lua": `return 42`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("runtime did not shut down")
	}
}

func TestRuntimeRejectsSyntaxAndEscapingModules(t *testing.T) {
	for _, source := range []string{`function broken(`, `require("../outside")`, `require("escape")`} {
		t.Run(source, func(t *testing.T) {
			dir := t.TempDir()
			outside := filepath.Join(t.TempDir(), "outside.lua")
			if err := os.WriteFile(outside, []byte(`return 42`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(dir, "escape.lua")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			if r, err := Open(t.Context(), dir, config.Default(), nil); err == nil {
				r.Close()
				t.Fatal("invalid candidate accepted")
			}
		})
	}
}
