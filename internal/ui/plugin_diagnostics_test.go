package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaStartupPrintUsesWorkspaceNotice(t *testing.T) {
	a := pluginFixture(t, `print("loaded", "module\nready\27[2J")`)
	app, err := New(a.Config, a.Paths, a.Session, false, a.plugins)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)

	if !strings.Contains(app.notice, "lua: loaded module ready") {
		t.Fatalf("startup diagnostics missing: %q", app.notice)
	}
	if strings.ContainsAny(app.notice, "\n\x1b") {
		t.Fatalf("startup diagnostic contains terminal controls: %q", app.notice)
	}
}

func TestLuaReloadAndCallbackPrintUseNotices(t *testing.T) {
	a := pluginFixture(t, "")
	if err := config.Save(config.File(a.Paths.Config), a.Config); err != nil {
		t.Fatal(err)
	}
	source := `print("reloaded")
wraith.commands.create("print-test", function() print("callback", 42) end)`
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	a.Update(a.reloadPlugins()())
	if !strings.Contains(a.notice, "lua: reloaded") {
		t.Fatalf("reload diagnostics missing: %q", a.notice)
	}

	a.Update(a.runCommand("print-test")())
	if !strings.Contains(a.notice, "callback") || !strings.Contains(a.notice, "42") {
		t.Fatalf("callback print did not reach notice: %q", a.notice)
	}
}
