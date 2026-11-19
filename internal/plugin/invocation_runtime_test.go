package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func beginTestInvocation(t *testing.T, r *Runtime, data string) (*Invocation, InvocationStart) {
	t.Helper()
	definition := r.Manifest().Providers[0]
	binding := InvocationBinding{"provider", definition.Name, "chat", definition.Owner, definition.Lease, definition.Chat, definition.Revision}
	call, err := r.BeginInvocation(t.Context(), InvocationRequest{Binding: binding, Data: json.RawMessage(data)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(call.Close)
	select {
	case start := <-r.InvocationStarts():
		return call, start
	case <-time.After(time.Second):
		t.Fatal("invocation did not request dispatch")
		return nil, InvocationStart{}
	}
}

func TestInvocationDefersRepliesAndPreservesJSONTypes(t *testing.T) {
	r := openTestRuntime(t, `
wraith.providers.register("custom",{models=function() end,show=function() end,chat=function(ctx)
 assert(ctx.prompt=="fresh" and ctx.invocation.method=="chat")
 assert(wraith.json.encode(ctx.data)=='{"array":[],"object":{},"value":null}')
 local id=ctx.invocation.id
 wraith.invocation.emit(id,{content="first"})
 wraith.schedule(function(next)
  assert(next.invocation.id==id)
  wraith.invocation.finish(id,{array=wraith.json.array({}),object=wraith.json.object({}),value=wraith.json.null})
 end)
end})`)
	call, start := beginTestInvocation(t, r, `{"array":[],"object":{},"value":null}`)
	pending, err := r.SubmitInvocation(start, map[string]any{"prompt": "fresh"}, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Invocation != call.ID || len(result.Effects) != 2 {
		t.Fatalf("invocation dispatch: %+v", result)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	if _, err := call.Next(ctx); err == nil {
		t.Fatal("reply escaped host acceptance")
	}
	if err := r.CommitInvocation(result.Effects[0].Data.(InvocationEffect)); err != nil {
		t.Fatal(err)
	}
	chunk, err := call.Next(t.Context())
	if err != nil || string(chunk.Data) != `{"content":"first"}` || chunk.Final {
		t.Fatalf("stream delta: %+v, %v", chunk, err)
	}
	pending, err = r.Submit(result.Effects[1].Argument, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	deferred := receiveResult(t, pending)
	if deferred.Err != nil || deferred.Invocation != call.ID || deferred.Effects[0].Invocation != call.ID {
		t.Fatalf("deferred invocation lost its identity: %+v", deferred)
	}
	if err := r.CommitInvocation(deferred.Effects[0].Data.(InvocationEffect)); err != nil {
		t.Fatal(err)
	}
	final, err := call.Next(t.Context())
	if err != nil || !final.Final || string(final.Data) != `{"array":[],"object":{},"value":null}` || r.InvocationValid(call.ID) {
		t.Fatalf("final reply: %+v, %v", final, err)
	}
}

func TestInvocationRetainsAcceptedCallbackDuringReplacement(t *testing.T) {
	r := openTestRuntime(t, `
local function install(value,replace)
 wraith.providers.register("custom",{replace=replace,models=function() end,show=function() end,chat=function(ctx)
  wraith.invocation.finish(ctx.invocation.id,value)
 end})
end
install("old",false)
wraith.commands.create("replace",function() install("new",true) end)
`)
	manifest := r.Manifest()
	pending, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	replacement := receiveResult(t, pending)
	if replacement.Err != nil || replacement.Manifest == nil {
		t.Fatalf("replacement failed: %+v", replacement)
	}
	call, start := beginTestInvocation(t, r, `null`)
	if err := r.Acknowledge(replacement.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.BeginInvocation(t.Context(), InvocationRequest{Binding: start.Binding}); err == nil {
		t.Fatal("stale definition accepted a new call")
	}
	pending, err = r.SubmitInvocation(start, nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 1 {
		t.Fatalf("bound implementation disappeared: %+v", result)
	}
	if err := r.CommitInvocation(result.Effects[0].Data.(InvocationEffect)); err != nil {
		t.Fatal(err)
	}
	final, err := call.Next(t.Context())
	if err != nil || string(final.Data) != `"old"` {
		t.Fatalf("bound call changed implementation: %+v, %v", final, err)
	}
}
