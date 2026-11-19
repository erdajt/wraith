package plugin

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestManagerObserverSaturationPreservesCompletion keeps lifecycle release ahead of best-effort diagnostics.
func TestManagerObserverSaturationPreservesCompletion(t *testing.T) {
	directory := t.TempDir()
	r := openTestRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 local completed=false
 store.execute((store.plan("install")),function(result)
  if result.state=="installed" then for i=1,65 do wraith.notify("observer notice") end end
 end,function(result) completed=result.state=="installed" end)
 wraith.commands.create("retry",function()
  assert(completed and store.status().state=="installed")
  store.execute((store.plan("install")),nil,function(result)
   assert(result.state=="installed",result.error)
   wraith.notify("complete")
  end)
 end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json")))
	if err := r.Activate(r.Manifest().Resources); err != nil {
		t.Fatal(err)
	}
	for {
		event := receiveResource(t, r)
		if !r.ResourceValid(event) {
			continue
		}
		pending, err := r.SubmitResource(event, nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		notices := 0
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			case "notify":
				notices++
			default:
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
		if notices > 60 {
			break
		}
	}
	r.services.mu.Lock()
	remaining := len(r.services.records)
	r.services.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("completion retained %d resources", remaining)
	}
	var callback string
	for _, command := range r.Manifest().Commands {
		if command.Name == "retry" {
			callback = command.Callback
		}
	}
	pending, err := r.Submit(callback, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	for _, effect := range result.Effects {
		if effect.Name != "resource.activate" {
			t.Fatalf("unexpected effect: %+v", effect)
		}
		if err := r.Activate([]string{effect.Argument}); err != nil {
			t.Fatal(err)
		}
	}
	runManagerEvents(t, r)
}
