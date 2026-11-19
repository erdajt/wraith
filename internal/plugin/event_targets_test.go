package plugin

import "testing"

func TestTargetedEventsPreservePayloadAndRejectInvalidSubscribers(t *testing.T) {
	r := openTestRuntime(t, `local first = wraith.events.on("ready", function(ctx) wraith.notify(ctx.value) end)
local other = wraith.events.on("window_changed", function() end)
wraith.commands.create("deliver", function()
 local listeners = wraith.events.listeners("ready")
 local data = {value="original"}
 wraith.events.dispatch("ready", data, {listeners=listeners})
 data.value, listeners[1] = "changed", other
 for _, ids in ipairs({{other}, {first,first}, {[2]=first}, {first, extra=first}}) do
  assert(not pcall(wraith.events.dispatch,"ready",{}, {listeners=ids}))
 end
 assert(not pcall(wraith.events.dispatch,"ready",{}, {listeners={}, extra=true}))
 assert(not pcall(wraith.events.listeners,"unknown"))
end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 {
		t.Fatalf("targeted effect: %+v", got)
	}
	delivery := got.Effects[0].Data.(EventDelivery)
	pending, err = r.SubmitEvent("ready", delivery.Listeners, delivery.Data, m.Config, "")
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "original" {
		t.Fatalf("detached delivery: %+v", got)
	}
}

func TestTargetedEventsKeepEmptyAndRetiredSetsWithoutCoalescing(t *testing.T) {
	r := openTestRuntime(t, `local listener = wraith.events.on("window_changed", function(ctx) wraith.notify(ctx.value) end)
wraith.commands.create("hold", function()
 wraith.commands.create("registered",function() end)
 wraith.events.dispatch("window_changed",{}, {listeners={listener}})
end)
wraith.commands.create("remove",function() wraith.events.off(listener) end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	held := receiveResult(t, pending)
	if held.Err != nil || held.Manifest == nil || len(held.Effects) != 1 {
		t.Fatalf("publication barrier: %+v", held)
	}
	ids := held.Effects[0].Data.(EventDelivery).Listeners
	var replies []<-chan Result
	for _, value := range []string{"first", "second"} {
		reply, err := r.SubmitEvent("window_changed", ids, map[string]any{"value": value}, m.Config, "")
		if err != nil {
			t.Fatal(err)
		}
		replies = append(replies, reply)
	}
	if err := r.Acknowledge(held.Revision, true); err != nil {
		t.Fatal(err)
	}
	for i, reply := range replies {
		got := receiveResult(t, reply)
		if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != []string{"first", "second"}[i] {
			t.Fatalf("targeted event %d coalesced: %+v", i, got)
		}
	}

	for _, targets := range [][]string{nil, {}} {
		pending, err = r.SubmitEvent("window_changed", targets, nil, m.Config, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := receiveResult(t, pending); got.Err != nil || len(got.Effects) != 0 {
			t.Fatalf("empty target set broadcast: %+v", got)
		}
	}

	pending, err = r.Submit(m.Commands[1].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	removed := receiveResult(t, pending)
	if removed.Err != nil || removed.Manifest == nil || r.Acknowledge(removed.Revision, true) != nil {
		t.Fatalf("remove listener: %+v", removed)
	}
	for _, targets := range [][]string{nil, {}, ids} {
		pending, err = r.SubmitEvent("window_changed", targets, nil, m.Config, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := receiveResult(t, pending); got.Err != nil || len(got.Effects) != 0 {
			t.Fatalf("empty or retired delivery broadcast: %+v", got)
		}
	}
}
