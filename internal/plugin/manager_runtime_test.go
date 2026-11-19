package plugin

import "testing"

const managerLocalStore = `package.preload["wraith.manager.store"]=function() return {new=function(specs)
 local rows={}; for _,spec in ipairs(specs) do rows[#rows+1]={name=spec.name,source=spec.source,path=spec.source.dir,state="local"} end
 return {inspect=function() return rows end,status=function() end,cancel=function() end}
 end} end
`

func TestManagerSetupCanRetryAfterHostRejectsPublication(t *testing.T) {
	r := openManagerRuntime(t, managerLocalStore+`
 package.preload.app=function() return {setup=function() wraith.commands.create("app",function() wraith.notify("loaded") end) end} end
 wraith.commands.create("configure",function()
  require("wraith.manager").setup({{name="app",main="app",dir="/tmp/app",opts={},cmd="app"}})
 end)
 `)
	configure := r.Manifest().Commands[0].Callback
	for _, accepted := range []bool{false, true} {
		pending, err := r.Submit(configure, "", nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil || result.Manifest == nil {
			t.Fatalf("cached manager could not configure: %+v", result)
		}
		if err := r.Acknowledge(result.Revision, accepted); err != nil {
			t.Fatal(err)
		}
	}
	var app string
	for _, command := range r.Manifest().Commands {
		if command.Name == "app" {
			app = command.Callback
		}
	}
	pending, err := r.Submit(app, "", map[string]any{"argument": ""}, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Name != "command.app" {
		t.Fatalf("retry left stale manager or package wrappers: %+v", result)
	}
	_ = r.Acknowledge(result.Revision, true)
}

func TestManagerValidatesViewBeforeInitializingPackages(t *testing.T) {
	openManagerRuntime(t, managerLocalStore+`
 local initialized=0
 local specs={{name="app",dir="/tmp/app",lazy=true,init=function() initialized=initialized+1 end}}
 local manager=require("wraith.manager")
 for _,ui in ipairs({false,{position=false},{keys=false},{keys={refresh="<InvalidKey>"}}}) do
  assert(not pcall(manager.setup,specs,{ui=ui}))
  assert(initialized==0 and #wraith.plugins.list()==0)
 end
 manager.setup(specs)
 assert(initialized==1,"initialized="..initialized)
 assert(not pcall(manager.setup,specs))
 assert(manager.status().packages[1].managed,"not managed")
 manager.unload_all()
 wraith.plugins.unload("wraith.manager.runtime")
 manager.setup(specs)
 assert(initialized==2)
 `)
}

func TestManagerRejectsInvalidOptionsWithoutPublishingPartialSetup(t *testing.T) {
	r := openManagerRuntime(t, managerLocalStore+`
 local attempts=0
 package.preload.app=function() return {setup=function(opts)
  assert(opts.value=="ready")
  wraith.commands.create("app.ready",function() end)
 end} end
 local manager=require("wraith.manager")
 manager.setup({{name="app",main="app",dir="/tmp/app",lazy=true,opts=function(_,opts)
  attempts=attempts+1
  if attempts==1 then
   wraith.commands.create("partial",function() end)
   return false
  end
  opts.value="ready"
 end}})
 assert(not pcall(manager.load,"app"))
 assert(not manager.status().packages[1].loaded)
 manager.load("app")
 assert(manager.status().packages[1].loaded and attempts==2)
 `)
	for _, command := range r.Manifest().Commands {
		if command.Name == "partial" {
			t.Fatal("invalid options left a partial command registered")
		}
	}
}

func TestManagerLazyEventsReplayOnlyNewListenersAfterPublication(t *testing.T) {
	r := openManagerRuntime(t, managerLocalStore+`
 wraith.events.on("test.load",function(ctx) wraith.notify("existing:"..ctx.data.value) end)
 package.preload.app=function() return {setup=function()
  wraith.events.on("test.load",function(ctx) wraith.notify("new:"..ctx.data.value) end)
 end} end
 require("wraith.manager").setup({{name="app",main="app",dir="/tmp/app",opts={},event="test.load"}})
 `)
	pending, err := r.Submit("", "test.load", map[string]any{"data": map[string]any{"value": "original"}}, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 2 || result.Effects[0].Argument != "existing:original" || result.Effects[1].Name != "schedule.callback" {
		t.Fatalf("original event dispatch changed: %+v", result)
	}
	pending, err = r.Submit(result.Effects[1].Argument, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, pending)
	if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Name != "event.dispatch" {
		t.Fatalf("lazy observer registration was not published first: %+v", result)
	}
	delivery := result.Effects[0].Data.(EventDelivery)
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
	pending, err = r.SubmitEvent("test.load", delivery.Listeners, map[string]any{"data": delivery.Data}, r.Manifest().Config, "")
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != "new:original" {
		t.Fatalf("first event was lost or existing listener repeated: %+v", result)
	}
}
