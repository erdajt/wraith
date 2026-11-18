package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentedLuaConfigurationLoadsAndRuns(t *testing.T) {
	a := fixture(t)
	if err := os.CopyFS(a.Paths.Config, os.DirFS(filepath.Join("..", "..", "docs", "examples", "lua"))); err != nil {
		t.Fatal(err)
	}
	r, err := LoadPlugin(t.Context(), a.Paths.Config, a.Config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	a.plugins, a.pluginManifest = r, r.Manifest()
	a.Config, a.actions = a.pluginManifest.Config, a.pluginActions(a.pluginManifest)
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.Update(a.runCommand("review.draft this package")())
	if a.Prompt.String() != "review this package for correctness and missing tests" {
		t.Fatalf("example command failed: %q", a.Prompt.String())
	}
	a.width, a.height, a.focus = 120, 40, "chat"
	a.Update(a.runCommand("review.layout")())
	if len(a.windows) != 2 || a.windowID != 1 {
		t.Fatal("example layout did not split and refocus")
	}
	if a.Config.Theme != "rose-pine" || a.Config.Themes["nightwatch"].Accent != "#8fbcbb" {
		t.Fatal("example theme settings were not applied")
	}
}
