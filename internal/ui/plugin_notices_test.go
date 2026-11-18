package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaCleanupNoticeReachesWorkspace(t *testing.T) {
	a := pluginFixture(t, `return function() error("cleanup failed") end`)
	wait := a.watchPluginNotices(a.plugins)
	a.plugins.Close()
	result := make(chan pluginNoticeMsg, 1)
	go func() {
		if message, ok := wait().(pluginNoticeMsg); ok {
			result <- message
		}
	}()

	select {
	case message := <-result:
		a.Update(message)
		if !strings.Contains(a.notice, "cleanup failed") {
			t.Fatalf("cleanup error missing: %q", a.notice)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup error did not reach the workspace")
	}
}

func TestLuaNoticeLabelsRetiredRuntimeAndStripsControls(t *testing.T) {
	a := pluginFixture(t, "")
	retired := pluginFixture(t, "")
	a.notice = "current"
	if cmd := a.applyPluginNotice(pluginNoticeMsg{Runtime: retired.plugins, Text: "retired"}); cmd == nil || !strings.Contains(a.notice, "previous configuration cleanup") {
		t.Fatal("retired runtime diagnostic lost its origin")
	}

	a.applyPluginNotice(pluginNoticeMsg{Runtime: a.plugins, Text: "cleanup\nfailed\x1b[2J"})
	if strings.ContainsAny(a.notice, "\n\x1b") || !strings.Contains(a.notice, "cleanup failed") {
		t.Fatalf("unsafe or missing diagnostic: %q", a.notice)
	}
}

func TestLuaReloadReportsPreviousCleanupFailure(t *testing.T) {
	a := pluginFixture(t, `return function() error("release failed") end`)
	previous := a.plugins
	wait := a.watchPluginNotices(previous)
	if err := config.Save(config.File(a.Paths.Config), a.Config); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(`wraith.commands.create("current", function() end)`), 0600); err != nil {
		t.Fatal(err)
	}
	a.Update(a.reloadPlugins()())
	select {
	case <-previous.Done():
	case <-time.After(time.Second):
		t.Fatal("previous runtime did not finish cleanup")
	}
	a.Update(wait())
	if a.plugins == previous || !strings.Contains(a.notice, "previous configuration cleanup") || !strings.Contains(a.notice, "release failed") {
		t.Fatalf("reload lost cleanup failure: %q", a.notice)
	}
	if len(a.pluginDiagnostics) != 1 {
		t.Fatal("reload diagnostic was not retained")
	}
}

func TestLuaNoticeWaitEndsWithWorkspace(t *testing.T) {
	a := pluginFixture(t, "")
	wait := a.watchPluginNotices(a.plugins)
	a.Close()
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("notice watcher survived workspace shutdown")
	}
}
