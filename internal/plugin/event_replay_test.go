package plugin

import (
	"slices"
	"testing"
)

// TestReadyReplayKeepsExplicitDeliveriesAndOwners filters only earlier catchup requests.
func TestReadyReplayKeepsExplicitDeliveriesAndOwners(t *testing.T) {
	e := moduleResolutionFixture(t)
	e.readyStarted = true
	if err := e.L.DoString(`
first=wraith.events.on("ready",function() end)
second=wraith.events.on("ready",function() end)
third=wraith.events.on("ready",function() end)
custom=wraith.events.on("test.changed",function() end)
ids={first,second}
wraith.events.replay_ready(ids)
ids[1]=third
`); err != nil {
		t.Fatal(err)
	}
	before := e.captureRegistry()
	e.owner = "core"
	if err := e.L.DoString(`
wraith.events.replay_ready({second,third})
wraith.events.replay_ready({third,first})
wraith.events.dispatch("ready",{}, {listeners={first}})
wraith.events.dispatch("test.changed",{}, {listeners={custom}})
wraith.events.dispatch("test.changed",{}, {listeners={custom}})
`); err != nil {
		t.Fatal(err)
	}
	if len(e.effects) != 5 {
		t.Fatalf("replay changed explicit delivery count: %+v", e.effects)
	}
	first := e.effects[0].Data.(EventDelivery)
	second := e.effects[1].Data.(EventDelivery)
	if !first.replayReady || !second.replayReady || first.Data != nil || second.Data != nil ||
		!slices.Equal(first.Listeners, []string{e.L.GetGlobal("first").String(), e.L.GetGlobal("second").String()}) ||
		!slices.Equal(second.Listeners, []string{e.L.GetGlobal("third").String()}) {
		t.Fatal("replay did not preserve first occurrence order and nil payload")
	}
	if e.effects[0].Owner != "config" || e.effects[1].Owner != "core" || e.effects[0].Lease == e.effects[1].Lease {
		t.Fatal("replay merged distinct owner leases")
	}
	if !slices.Equal(before.effects[0].Data.(EventDelivery).Listeners, first.Listeners) {
		t.Fatal("later replay mutated a saved effect")
	}
	for _, effect := range e.effects[2:] {
		if effect.Data.(EventDelivery).replayReady {
			t.Fatal("explicit dispatch acquired catchup semantics")
		}
	}
}

// TestReadyReplayValidatesBoundsBeforeStartupNoop rejects malformed and retired subscriber lists.
func TestReadyReplayValidatesBoundsBeforeStartupNoop(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`
assert(wraith.capabilities.ready_replay)
ids={}
for index=1,32 do ids[index]=wraith.events.on("ready",function() end) end
other=wraith.events.on("window_changed",function() end)
for _,bad in ipairs({{other},{ids[1],ids[1]},{[2]=ids[1]},{ids[1],extra=ids[2]},
 {false},{1},{[1.5]=ids[1]},{"retired"}}) do
 assert(not pcall(wraith.events.replay_ready,bad))
end
local too_many={}
for index=1,33 do too_many[index]=ids[1] end
assert(not pcall(wraith.events.replay_ready,too_many))
assert(not pcall(wraith.events.replay_ready))
wraith.events.replay_ready(ids)
`); err != nil {
		t.Fatal(err)
	}
	if len(e.effects) != 0 {
		t.Fatal("catchup ran before the initial ready event")
	}
	e.readyStarted, e.dirty = true, false
	if err := e.L.DoString(`wraith.events.replay_ready({}); wraith.events.replay_ready(ids)`); err != nil {
		t.Fatal(err)
	}
	if e.dirty || len(e.effects) != 1 || len(e.effects[0].Data.(EventDelivery).Listeners) != 32 {
		t.Fatal("inclusive replay bound changed registry publication")
	}
}

// TestReadyReplayCapturedGuards rejects empty requests in restricted execution phases.
func TestReadyReplayCapturedGuards(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`captured=wraith.events.replay_ready`); err != nil {
		t.Fatal(err)
	}
	e.setup, e.dirty = true, false
	if err := e.L.DoString(`captured({})`); err != nil || e.dirty || len(e.effects) != 0 {
		t.Fatal("startup empty replay changed state", err)
	}
	e.setup, e.pureHook = false, true
	if err := e.L.DoString(`assert(not pcall(captured,{}))`); err != nil || e.hookViolation == nil {
		t.Fatal("caught replay bypassed pure hook guard", err)
	}
	e.pureHook, e.hookViolation, e.closing = false, nil, true
	if err := e.L.DoString(`assert(not pcall(captured,{}))`); err != nil {
		t.Fatal(err)
	}
	e.closing = false
	if err := e.L.DoString(`wraith.modules.provider(function() pcall(captured,{}); return nil end)`); err != nil {
		t.Fatal(err)
	}
	e.dirty = false
	if _, err := e.resolveModuleProvider(e.L, "probe"); err == nil || e.dirty || len(e.effects) != 0 {
		t.Fatal("caught replay bypassed metadata guard", err)
	}
}

// TestReadyReplayTargetedDispatchDoesNotStartLifecycle distinguishes explicit delivery from native ready.
func TestReadyReplayTargetedDispatchDoesNotStartLifecycle(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`listener=wraith.events.on("ready",function() end)`); err != nil {
		t.Fatal(err)
	}
	targeted := request{event: "ready", data: []byte("null"), config: e.manifest.Config, listeners: []string{e.L.GetGlobal("listener").String()}}
	if result := e.invoke(t.Context(), targeted); result.Err != nil || e.readyStarted {
		t.Fatal("explicit ready delivery started the native lifecycle", result.Err)
	}
	targeted.listeners = nil
	if result := e.invoke(t.Context(), targeted); result.Err != nil || !e.readyStarted || result.Manifest != nil {
		t.Fatal("native ready did not start without a registry publication", result.Err)
	}
}
