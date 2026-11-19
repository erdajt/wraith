package plugin

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestManagerStorageRecoversDiscardedActivation permits a new plan after host rejection removed its resource.
func TestManagerStorageRecoversDiscardedActivation(t *testing.T) {
	directory := t.TempDir()
	r := openManagerRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 local first
 wraith.commands.create("begin",function()
  first=store.execute((store.plan("install")))
 end)
 wraith.commands.create("retry",function()
  assert(store.status().state=="failed")
  assert(not store.cancel(first))
  store.execute((store.plan("install")),nil,function(result)
   assert(result.state=="installed",result.error)
   wraith.notify("complete")
  end)
 end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json")))
	for _, name := range []string{"begin", "retry"} {
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
		for _, effect := range result.Effects {
			if effect.Name != "resource.activate" {
				t.Fatalf("unexpected effect: %+v", effect)
			}
			if name == "begin" {
				r.DiscardResources([]string{effect.Argument})
			} else if err := r.Activate([]string{effect.Argument}); err != nil {
				t.Fatal(err)
			}
		}
	}
	runManagerResources(t, r)
}

// TestManagerStorageClearsSynchronousReservationFailures permits retry after the worker limit is released.
func TestManagerStorageClearsSynchronousReservationFailures(t *testing.T) {
	directory := t.TempDir()
	r := openManagerRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 wraith.commands.create("saturate",function()
  for i=1,32 do wraith.timer.start(3600000,function() end) end
  local plan=store.plan("install")
  assert(not pcall(store.execute,plan))
  assert(store.status().state=="failed")
 end)
 wraith.commands.create("retry",function()
  local plan=store.plan("install")
  store.execute(plan,nil,function(result)
   assert(result.state=="installed",result.error)
   wraith.notify("complete")
  end)
 end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json")))
	for _, name := range []string{"saturate", "retry"} {
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
		for _, effect := range result.Effects {
			if effect.Name != "resource.activate" {
				t.Fatalf("unexpected effect: %+v", effect)
			}
			if name == "saturate" {
				r.DiscardResources([]string{effect.Argument})
			} else if err := r.Activate([]string{effect.Argument}); err != nil {
				t.Fatal(err)
			}
		}
	}
	runManagerResources(t, r)
}

// TestManagerStorageAbandonsFailedNestedCallbacks releases rolled-back source and child receipts before retry.
func TestManagerStorageAbandonsFailedNestedCallbacks(t *testing.T) {
	directory := t.TempDir()
	r := openManagerRuntime(t, fmt.Sprintf(`local mkdir=wraith.fs.mkdir_async
 local failed=false
 wraith.fs.mkdir_async=function(path,options,callback)
  return mkdir(path,options,function(ctx)
   callback(ctx)
   if ctx.resource.final and not failed then failed=true; error("reject nested continuation") end
  end)
 end
 local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 store.execute((store.plan("install")))
 wraith.commands.create("recover",function() assert(store.status().state=="failed") end)
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
	for _, name := range []string{"recover", "retry"} {
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
		for _, effect := range result.Effects {
			if effect.Name == "resource.release" {
				r.DiscardResources([]string{effect.Argument})
			} else if effect.Name == "resource.activate" {
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
		if name == "recover" {
			r.services.mu.Lock()
			remaining := len(r.services.records)
			r.services.mu.Unlock()
			if remaining != 0 {
				t.Fatalf("failed chain retained %d resources", remaining)
			}
		}
	}
	runManagerEvents(t, r)
}
