package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

// TestDialogDecisionRunsOnce keeps consumed callbacks out of registry rollback and replay.
func TestDialogDecisionRunsOnce(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("choose", function()
 wraith.ui.select({{id="scope",label="target"}}, {title="choose scope"}, function(ctx)
  assert(ctx.data.item_id == "scope" and ctx.data.cancelled == false)
  wraith.notify(ctx.data.value)
 end)
end)`)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, ch)
	if result.Err != nil || len(result.Effects) != 1 {
		t.Fatalf("open dialog: %+v", result)
	}
	dialog := result.Effects[0].Data.(Dialog)
	if !r.Surfaces().OwnerValid(dialog.Owner, dialog.Lease) {
		t.Fatal("dialog owner is invalid")
	}
	snapshot := map[string]any{"data": map[string]any{"item_id": "scope", "value": "target", "cancelled": false}}
	for attempt := range 2 {
		ch, err = r.Submit(dialog.ID, "", snapshot, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		result = receiveResult(t, ch)
		if result.Err != nil || len(result.Effects) != 1-attempt {
			t.Fatalf("decision %d: %+v", attempt, result)
		}
	}
}

// TestCaughtDialogPublicationFailureReleasesCallback covers failures caught inside user Lua.
func TestCaughtDialogPublicationFailureReleasesCallback(t *testing.T) {
	dir := t.TempDir()
	source := `for i=1,64 do wraith.schedule(function() end) end
assert(not pcall(function() wraith.ui.input({},function() error("abandoned") end) end))`
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := load(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.L.Close()
	defer e.services.close()
	defer e.surfaces.close()
	if len(e.scheduled) != 64 {
		t.Fatalf("pending callbacks = %d, want 64", len(e.scheduled))
	}
}

// TestDialogCallbackFailureStillConsumesDecision prevents a failed callback from being replayed.
func TestDialogCallbackFailureStillConsumesDecision(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("input",function()
 wraith.ui.input({},function() error("failed decision") end)
end)`)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	dialog := receiveResult(t, ch).Effects[0].Data.(Dialog)
	for attempt := range 2 {
		ch, err = r.Submit(dialog.ID, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, ch)
		if (result.Err != nil) != (attempt == 0) || len(result.Effects) != 0 {
			t.Fatalf("decision %d: %+v", attempt, result)
		}
	}
}
