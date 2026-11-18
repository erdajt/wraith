package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
)

func openTestRuntime(t *testing.T, source string) *Runtime {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

func receiveResult(t *testing.T, result <-chan Result) Result {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(3 * time.Second):
		t.Fatal("callback result did not arrive")
		return Result{}
	}
}

func TestPublicationRequiresHostDecisionBeforeDependentCallback(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("load", function()
 wraith.commands.create("child", function() wraith.notify("child") end)
end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	change := receiveResult(t, pending)
	if change.Err != nil || change.Manifest == nil || len(change.Manifest.Commands) != 2 {
		t.Fatalf("publication: %+v", change)
	}
	if len(r.Manifest().Commands) != 1 {
		t.Fatal("unacknowledged registry became visible")
	}
	if err := r.Acknowledge(change.Revision, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Acknowledge(change.Revision, true); err == nil {
		t.Fatal("duplicate acknowledgement accepted")
	}
	m = r.Manifest()
	child, err := r.Submit(m.Commands[1].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, child)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "child" {
		t.Fatalf("dependent callback: %+v", got)
	}
}

func TestRejectedAndFailedRegistrationRestorePreviousCallbacks(t *testing.T) {
	for _, mode := range []string{"reject", "error"} {
		t.Run(mode, func(t *testing.T) {
			r := openTestRuntime(t, `wraith.commands.create("change", function(ctx)
 wraith.commands.create("old", function() wraith.notify("new") end, "new", {replace=true})
 if ctx.mode == "error" then error("rollback") end
end)
wraith.commands.create("old", function() wraith.notify("old") end)`)
			m := r.Manifest()
			pending, err := r.Submit(m.Commands[0].Callback, "", map[string]any{"mode": mode}, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			got := receiveResult(t, pending)
			if mode == "reject" {
				if got.Manifest == nil || r.Acknowledge(got.Revision, false) != nil {
					t.Fatalf("rejected publication: %+v", got)
				}
			} else if got.Err == nil || got.Manifest != nil {
				t.Fatalf("failed registration survived: %+v", got)
			}
			old, err := r.Submit(m.Commands[1].Callback, "", nil, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			got = receiveResult(t, old)
			if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "old" {
				t.Fatalf("original callback lost: %+v", got)
			}
		})
	}
}

func TestCloseCancelsWaitingRegistryPublication(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("load", function()
 wraith.commands.create("child", function() end)
end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	change := receiveResult(t, pending)
	if change.Manifest == nil {
		t.Fatalf("missing publication: %+v", change)
	}
	r.Close()
	if err := r.Acknowledge(change.Revision, true); err == nil {
		t.Fatal("closed runtime accepted publication")
	}
	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("publication blocked shutdown")
	}
}
