package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestFailedLuaReloadKeepsWorkingCallbacksAndSettings(t *testing.T) {
	a := pluginFixture(t, `wraith.config.set("theme", "nord"); wraith.commands.create("check", function() wraith.notify("working") end)`)
	a.demo = true
	before := a.plugins
	if err := config.Save(config.File(a.Paths.Config), config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(`wraith.config.set("context", -1)`), 0600); err != nil {
		t.Fatal(err)
	}
	a.Update(a.reloadPlugins()())
	if a.plugins != before || a.Config.Theme != "nord" {
		t.Fatal("failed reload replaced working configuration")
	}
	a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
	if a.notice != "working" {
		t.Fatalf("failed reload stopped the old worker: %s", a.notice)
	}
}

func TestLuaReloadReplacesRegistrationsAndDropsOldResults(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("old", function() wraith.notify("old result") end)`)
	a.demo = true
	old := a.plugins
	queued := a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")
	if err := config.Save(config.File(a.Paths.Config), config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(`wraith.commands.create("current", function() wraith.notify("current result") end)`), 0600); err != nil {
		t.Fatal(err)
	}
	result := queued()
	a.Update(a.reloadPlugins()())
	a.Update(result)
	if a.plugins == old || len(a.pluginManifest.Commands) != 1 || a.pluginManifest.Commands[0].Name != "current" || a.notice == "old result" {
		t.Fatal("reload kept old callbacks or accepted stale effects")
	}
	a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
	if a.notice != "current result" {
		t.Fatalf("new callback failed: %s", a.notice)
	}
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(`error("broken")`), 0600); err != nil {
		t.Fatal(err)
	}
	a.Update(a.reloadPlugins()())
	a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
	if a.notice != "current result" {
		t.Fatal("second reload cancelled the installed runtime")
	}
}
