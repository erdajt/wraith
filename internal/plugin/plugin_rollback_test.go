package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFailedDynamicLoadReleasesNewResources(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "closed")
	r := openTestRuntime(t, fmt.Sprintf(`
 package.preload.resource = function() return {
  setup = function() end,
  teardown = function() local f=assert(io.open(%q,"w")); f:write("closed"); f:close() end
 } end
 wraith.commands.create("load",function() wraith.plugins.load("resource"); error("stop") end)
 `, marker))
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Err == nil {
		t.Fatal("expected callback failure")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("registered cleanup dropped on failed callback: %v", err)
	}
}

func TestCaughtPluginSetupFailureRestoresRegistrations(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.resource = function() return {
  setup = function() wraith.commands.create("leaked",function() end); error("broken plugin") end
 } end
 wraith.commands.create("load",function() assert(not pcall(wraith.plugins.load,"resource")) end)
 `)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Manifest != nil {
		defer r.Acknowledge(got.Revision, false)
		for _, command := range got.Manifest.Commands {
			if command.Name == "leaked" {
				t.Fatal("caught failed plugin load retains partial registrations")
			}
		}
	}
}

func TestRejectedLoadReportsCleanupFailureAfterAcknowledgement(t *testing.T) {
	r := openTestRuntime(t, `package.preload.resource = function() return {
 setup = function() end,
 teardown = function() error("cleanup failed") end
} end
wraith.commands.create("load", function() wraith.plugins.load("resource") end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	change := receiveResult(t, pending)
	if change.Manifest == nil {
		t.Fatalf("missing publication: %+v", change)
	}
	if err := r.Acknowledge(change.Revision, false); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-r.Notices():
		if !strings.Contains(message, "cleanup failed") {
			t.Fatalf("cleanup notice: %q", message)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup failure was not reported")
	}
	if len(r.Diagnostics()) != 1 {
		t.Fatal("cleanup diagnostic was not retained")
	}
}
