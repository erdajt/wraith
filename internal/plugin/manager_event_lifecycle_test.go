package plugin

import "testing"

func TestManagerQueuedEventsStayWithinTheirActivationCohort(t *testing.T) {
	for _, intervening := range []string{"app", "unrelated", "retired"} {
		t.Run(intervening, func(t *testing.T) {
			r := openManagerRuntime(t, managerLocalStore+`
 package.preload.app=function() return {setup=function()
  wraith.events.on("test.load",function(ctx) wraith.notify("app:"..ctx.data.value) end)
 end} end
 package.preload.unrelated=function() return {setup=function()
  wraith.events.on("test.load",function(ctx) wraith.notify("unrelated:"..ctx.data.value) end)
 end} end
 local manager=require("wraith.manager")
 manager.setup({{name="app",dir="/tmp/app",event="test.load"},{name="unrelated",dir="/tmp/unrelated",lazy=true}})
 wraith.commands.create("intervene",function(ctx)
  if ctx.argument=="retired" then manager.unload("app"); manager.load("app") else manager.load(ctx.argument) end
 end,"intervene",{arguments=true})
 `)
			var schedules []string
			for _, value := range []string{"first", "second"} {
				pending, err := r.Submit("", "test.load", map[string]any{"data": map[string]any{"value": value}}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result := receiveResult(t, pending)
				if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Name != "schedule.callback" {
					t.Fatalf("event was not deferred: %+v", result)
				}
				schedules = append(schedules, result.Effects[0].Argument)
			}
			var command string
			for _, item := range r.Manifest().Commands {
				if item.Name == "intervene" {
					command = item.Callback
				}
			}
			pending, err := r.Submit(command, "", map[string]any{"argument": intervening}, r.Manifest().Config)
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			if result.Err != nil || result.Manifest == nil || r.Acknowledge(result.Revision, true) != nil {
				t.Fatalf("intervening package transition: %+v", result)
			}
			for index, id := range schedules {
				pending, err = r.Submit(id, "", nil, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result = receiveResult(t, pending)
				if result.Err != nil {
					t.Fatal(result.Err)
				}
				if result.Manifest != nil {
					if err := r.Acknowledge(result.Revision, true); err != nil {
						t.Fatal(err)
					}
				}
				if intervening == "retired" {
					if len(result.Effects) != 0 {
						t.Fatalf("retired initialization received an old event: %+v", result)
					}
					continue
				}
				if len(result.Effects) != 1 || result.Effects[0].Name != "event.dispatch" {
					t.Fatalf("missing captured event: %+v", result)
				}
				delivery := result.Effects[0].Data.(EventDelivery)
				pending, err = r.SubmitEvent("test.load", delivery.Listeners, map[string]any{"data": delivery.Data}, r.Manifest().Config, "")
				if err != nil {
					t.Fatal(err)
				}
				result = receiveResult(t, pending)
				if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != []string{"app:first", "app:second"}[index] {
					t.Fatalf("event cohort or original payload changed: %+v", result)
				}
			}
		})
	}
}

func TestManagerEagerChainPublishesBeforeTargetedReadyDelivery(t *testing.T) {
	r := openManagerRuntime(t, managerLocalStore+`
 wraith.events.on("ready",function() wraith.notify("existing") end)
 for _,name in ipairs({"first","second"}) do
  package.preload[name]=function() return {setup=function()
   wraith.notify("setup:"..name)
   wraith.events.on("ready",function() wraith.notify("ready:"..name) end)
  end} end
 end
 require("wraith.manager").setup({
  {name="lazy",dir="/tmp/lazy",lazy=true,priority=100,dependencies={{name="second",dir="/tmp/second",lazy=false,priority=0}}},
  {name="first",dir="/tmp/first"},
 })
 `)
	pending, err := r.Submit("", "ready", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Manifest == nil || len(result.Effects) != 2 || result.Effects[0].Argument != "existing" || result.Effects[1].Name != "schedule.callback" {
		t.Fatalf("startup did not queue one eager activation: %+v", result)
	}
	next := result.Effects[1].Argument
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
	for index, name := range []string{"first", "second"} {
		pending, err = r.Submit(next, "", nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result = receiveResult(t, pending)
		if result.Err != nil || result.Manifest == nil || len(result.Effects) != 3-index || result.Effects[0].Argument != "setup:"+name || result.Effects[1].Name != "event.dispatch" {
			t.Fatalf("eager order or publication changed: %+v", result)
		}
		delivery := result.Effects[1].Data.(EventDelivery)
		if index == 0 {
			next = result.Effects[2].Argument
		}
		if err := r.Acknowledge(result.Revision, true); err != nil {
			t.Fatal(err)
		}
		pending, err = r.SubmitEvent("ready", delivery.Listeners, nil, r.Manifest().Config, "")
		if err != nil {
			t.Fatal(err)
		}
		result = receiveResult(t, pending)
		if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != "ready:"+name {
			t.Fatalf("ready observer was repeated or omitted: %+v", result)
		}
	}
}
