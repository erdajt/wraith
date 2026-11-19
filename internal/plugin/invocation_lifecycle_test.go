package plugin

import (
	"context"
	"testing"
)

func TestNestedInvocationEventsAttachAndCancelManagedChildren(t *testing.T) {
	r := openTestRuntime(t, `
wraith.providers.register("custom",{models=function() end,show=function() end,chat=function() end})
wraith.events.on("plugin.child",function(ctx)
 assert(ctx.invocation.id)
 wraith.timer.start(60000,function() error("canceled worker ran") end)
 wraith.schedule(function() error("canceled scheduled callback ran") end)
end)
`)
	call, _ := beginTestInvocation(t, r, `null`)
	pending, err := r.SubmitWithinInvocation(call.ID, "", "plugin.child", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Invocation != call.ID || len(result.Effects) != 2 {
		t.Fatalf("nested event lost invocation: %+v", result)
	}
	resource := result.Effects[0].Argument
	if _, err := r.services.Status(resource, "config"); err != nil {
		t.Fatal(err)
	}
	r.AbortInvocation(call.ID, context.Canceled)
	if _, err := r.services.Status(resource, "config"); err == nil {
		t.Fatal("cancellation retained an unactivated child")
	}
	pending, err = r.Submit(result.Effects[1].Argument, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	if stale := receiveResult(t, pending); stale.Err != nil || len(stale.Effects) != 0 {
		t.Fatalf("canceled deferred work ran: %+v", stale)
	}
	if _, err := r.SubmitWithinInvocation(call.ID, "", "plugin.child", nil, r.Manifest().Config); err == nil {
		t.Fatal("canceled invocation accepted nested work")
	}
}

func TestInvocationManagedCompletionRetainsReplyContext(t *testing.T) {
	r := openTestRuntime(t, `
wraith.providers.register("custom",{models=function() end,show=function() end,chat=function(ctx)
 local id=ctx.invocation.id
 wraith.timer.start(1,function(completed)
  assert(completed.invocation.id==id and completed.resource.final)
  wraith.invocation.finish(id,{done=true})
 end)
end})`)
	call, start := beginTestInvocation(t, r, `null`)
	pending, err := r.SubmitInvocation(start, nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 1 {
		t.Fatalf("managed reservation: %+v", result)
	}
	if err := r.Activate([]string{result.Effects[0].Argument}); err != nil {
		t.Fatal(err)
	}
	completion := receiveResource(t, r)
	pending, err = r.SubmitResource(completion, nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, pending)
	if result.Err != nil || result.Invocation != call.ID || len(result.Effects) != 1 {
		t.Fatalf("completion lost invocation: %+v", result)
	}
	if err := r.CommitInvocation(result.Effects[0].Data.(InvocationEffect)); err != nil {
		t.Fatal(err)
	}
	final, err := call.Next(t.Context())
	if err != nil || !final.Final || string(final.Data) != `{"done":true}` {
		t.Fatalf("managed reply: %+v, %v", final, err)
	}
	if r.ResourceValid(completion) {
		t.Fatal("finished invocation retained child resource")
	}
}

func TestOwnerUnloadCancelsInvocationsOnlyAfterAcceptance(t *testing.T) {
	r := openTestRuntime(t, `
package.preload.extension=function() return {setup=function()
 wraith.providers.register("custom",{models=function() end,show=function() end,chat=function() end})
end} end
wraith.plugins.load("extension")
wraith.commands.create("unload",function() wraith.plugins.unload("extension") end)
`)
	call, _ := beginTestInvocation(t, r, `null`)
	for _, accept := range []bool{false, true} {
		manifest := r.Manifest()
		pending, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil || result.Manifest == nil || !r.InvocationValid(call.ID) {
			t.Fatalf("candidate unload canceled accepted work: %+v", result)
		}
		if err := r.Acknowledge(result.Revision, accept); err != nil {
			t.Fatal(err)
		}
		if r.InvocationValid(call.ID) == accept {
			t.Fatalf("unload decision %v did not control call lifetime", accept)
		}
	}
	terminal, err := call.Next(t.Context())
	if err != nil || !terminal.Final || terminal.Error == "" {
		t.Fatalf("unload lost terminal failure: %+v, %v", terminal, err)
	}
}

func TestNestedInvocationEventsAreNotCoalesced(t *testing.T) {
	r := openTestRuntime(t, `
wraith.providers.register("custom",{models=function() end,show=function() end,chat=function() end})
wraith.commands.create("hold",function() wraith.commands.create("registered",function() end) end)
wraith.events.on("window_changed",function(ctx) wraith.invocation.emit(ctx.invocation.id,ctx.value) end)
`)
	call, _ := beginTestInvocation(t, r, `null`)
	manifest := r.Manifest()
	pending, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	held := receiveResult(t, pending)
	var replies []<-chan Result
	for _, value := range []string{"first", "second"} {
		reply, err := r.SubmitWithinInvocation(call.ID, "", "window_changed", map[string]any{"value": value}, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		replies = append(replies, reply)
	}
	if held.Manifest == nil || r.Acknowledge(held.Revision, true) != nil {
		t.Fatal("registration barrier failed")
	}
	for i, reply := range replies {
		result := receiveResult(t, reply)
		if result.Err != nil || len(result.Effects) != 1 || result.Invocation != call.ID {
			t.Fatalf("nested event %d was coalesced or lost: %+v", i, result)
		}
	}
}
