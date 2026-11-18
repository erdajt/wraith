package plugin

import (
	"testing"
)

func TestLifecycleEventsSurviveRegistryPublication(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("load", function()
 wraith.commands.create("child", function() end)
end)
wraith.events.on("response_finished", function(ctx) wraith.notify(ctx.value) end)
wraith.events.on("plugin.review.selected", function(ctx) wraith.notify(ctx.value) end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	change := receiveResult(t, pending)
	if change.Manifest == nil {
		t.Fatalf("missing publication: %+v", change)
	}
	var deliveries []<-chan Result
	for _, event := range []string{"response_finished", "response_finished", "plugin.review.selected"} {
		result, err := r.Submit("", event, map[string]any{"value": event}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		deliveries = append(deliveries, result)
	}
	if err := r.Acknowledge(change.Revision, true); err != nil {
		t.Fatal(err)
	}
	for i, result := range deliveries {
		got := receiveResult(t, result)
		if got.Err != nil || len(got.Effects) != 1 {
			t.Fatalf("event %d was lost: %+v", i, got)
		}
	}
}

func TestRepeatedCommandReplacementReclaimsCallbacks(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("replace", function()
 wraith.commands.create("child", function() end, "child", {replace=true})
end)`)
	for i := range 600 {
		m := r.Manifest()
		pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		change := receiveResult(t, pending)
		if change.Err != nil || change.Manifest == nil || len(change.Manifest.Callbacks) != 2 {
			t.Fatalf("replacement %d: %+v", i, change)
		}
		if err := r.Acknowledge(change.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCustomEventPayloadIsDetachedAndNativeEventsCannotBeEmitted(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("emit", function()
 local data = {selected = "original"}
 wraith.events.emit("plugin.review.selected", data)
 data.selected = "changed"
 assert(not pcall(wraith.events.emit, "response_finished", {}))
end)`)
	m := r.Manifest()
	pending, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Name != "event.emit" {
		t.Fatalf("event effect: %+v", got)
	}
	if got.Effects[0].Data.(map[string]any)["selected"] != "original" {
		t.Fatalf("event payload changed after emission: %+v", got.Effects[0].Data)
	}
}
