package plugin

import (
	"testing"
)

func TestPackageCallsPreserveOwnerAndRollbackCaughtFailures(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.target=function() return {
  setup=function() end,
  install=function(value)
   wraith.commands.create("target-command",function() wraith.notify(value) end)
   return {value=value}
  end,
  fail=function()
   wraith.commands.create("partial",function() end)
   wraith.timer.start(10000,function() end)
   error("failed method")
  end
 } end
 wraith.plugins.load("target")
 package.loaded.target={install=function() error("wrong module") end}
 wraith.commands.create("configure",function()
  assert(not pcall(wraith.plugins.call,"target","setup"))
  assert(not pcall(wraith.plugins.call,"target","fail"))
  assert(wraith.plugins.call("target","install","value").value=="value")
  wraith.notify("caller")
 end)
 wraith.commands.create("remove",function() wraith.plugins.unload("target") end)
 `)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Owner != "config" {
		t.Fatalf("call rollback or caller ownership failed: %+v", result)
	}
	if len(result.Manifest.Commands) != 3 {
		t.Fatalf("partial method registrations leaked: %+v", result.Manifest.Commands)
	}
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
	m = r.Manifest()
	ch, err = r.Submit(m.Commands[1].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil || len(result.Manifest.Commands) != 2 {
		t.Fatalf("method registration did not belong to target owner: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
}

func TestPackageCallProtectsSuspendedOwnersAndBoundsRecursion(t *testing.T) {
	openTestRuntime(t, `
 package.preload.second=function() return {setup=function() end,
  check=function() assert(not pcall(wraith.plugins.unload,"first")) end
 } end
 package.preload.first=function() return {setup=function() end,
  check=function() wraith.plugins.call("second","check") end,
  recur=function() wraith.plugins.call("first","recur") end
 } end
 wraith.plugins.load("first")
 wraith.plugins.load("second")
 wraith.plugins.call("first","check")
 local ok,err=pcall(wraith.plugins.call,"first","recur")
 assert(not ok and string.find(err,"nesting exceeds"))
 wraith.plugins.unload("first")
 wraith.plugins.unload("second")
 `)
}

func TestPackageMethodDependencyRejectionRestoresGraph(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.first=function() return {setup=function() end,link=function()
  wraith.plugins.depend("second")
 end} end
 package.preload.second=function() return {setup=function() end} end
 wraith.plugins.load("first")
 wraith.plugins.load("second")
 wraith.commands.create("link",function() wraith.plugins.call("first","link") end)
 wraith.commands.create("remove",function() wraith.plugins.unload("second") end)
 `)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil || r.Acknowledge(result.Revision, false) != nil {
		t.Fatalf("link publication: %+v", result)
	}
	ch, err = r.Submit(m.Commands[1].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil {
		t.Fatalf("rejected dependency blocked unload: %+v", result)
	}
	_ = r.Acknowledge(result.Revision, true)
}

func TestPackageMethodsKeepInvocationChildrenAndTargetOwnership(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.worker=function() return {setup=function() end,run=function(id)
  wraith.schedule(function(ctx)
   assert(ctx.invocation.id==id)
   wraith.invocation.finish(id,{done=true})
  end)
 end} end
 wraith.plugins.load("worker")
 wraith.providers.register("custom",{models=function() end,show=function() end,chat=function(ctx)
  wraith.plugins.call("worker","run",ctx.invocation.id)
 end})
 `)
	call, start := beginTestInvocation(t, r, `{}`)
	pending, err := r.SubmitInvocation(start, nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 1 || result.Invocation != call.ID || result.Effects[0].Owner == "config" {
		t.Fatalf("method lost invocation or target ownership: %+v", result)
	}
	if result.Manifest != nil {
		if err := r.Acknowledge(result.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
	pending, err = r.Submit(result.Effects[0].Argument, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 1 || result.Invocation != call.ID {
		t.Fatalf("method continuation lost invocation: %+v", result)
	}
	if err := r.CommitInvocation(result.Effects[0].Data.(InvocationEffect)); err != nil {
		t.Fatal(err)
	}
	final, err := call.Next(t.Context())
	if err != nil || !final.Final {
		t.Fatalf("method invocation did not finish: %+v, %v", final, err)
	}
}
