package plugin

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestManagerRetainsCompletedReceiptsAcrossRejectedPublications retries cleanup until the host accepts its releases.
func TestManagerRetainsCompletedReceiptsAcrossRejectedPublications(t *testing.T) {
	directory := t.TempDir()
	r := openTestRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 store.execute((store.plan("install")),function(result)
  if result.state=="installed" then wraith.commands.create("installed",function() end) end
 end)
 wraith.commands.create("reject",function()
  assert(store.status().state=="installed")
  wraith.commands.create("publication",function() end)
 end)
 wraith.commands.create("drain",function() assert(store.status().state=="installed") end)
 wraith.commands.create("retry",function()
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
		if result.Manifest != nil {
			if err := r.Acknowledge(result.Revision, false); err != nil {
				t.Fatal(err)
			}
			break
		}
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			default:
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
	}
	for _, name := range []string{"reject", "reject", "drain", "retry"} {
		var callback string
		for _, command := range r.Manifest().Commands {
			if command.Name == name {
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
		if name == "reject" {
			if result.Manifest == nil {
				t.Fatal("missing publication to reject")
			}
			if err := r.Acknowledge(result.Revision, false); err != nil {
				t.Fatal(err)
			}
			r.services.mu.Lock()
			retained := len(r.services.records)
			r.services.mu.Unlock()
			if retained != 1 {
				t.Fatalf("rejected release retained %d resources", retained)
			}
			continue
		}
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			default:
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
		if name == "drain" {
			r.services.mu.Lock()
			retained := len(r.services.records)
			r.services.mu.Unlock()
			if retained != 0 {
				t.Fatalf("accepted release retained %d resources", retained)
			}
		}
	}
	runManagerEvents(t, r)
}
