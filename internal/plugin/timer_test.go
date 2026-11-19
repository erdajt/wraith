package plugin

import (
	"testing"
	"time"
)

func receiveResource(t *testing.T, runtime *Runtime) ResourceEvent {
	t.Helper()
	select {
	case event := <-runtime.Events():
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("managed resource did not report an event")
		return ResourceEvent{}
	}
}

func TestTimerWaitsForActivationAndDeliversCompletion(t *testing.T) {
	r := openTestRuntime(t, `wraith.timer.start(1, function(ctx)
 assert(ctx.resource.final and ctx.data.state == "completed")
 assert(ctx.data.result.tick == 1)
 wraith.notify("finished")
end)`)
	m := r.Manifest()
	if len(m.Resources) != 1 {
		t.Fatalf("timer reservation missing: %v", m.Resources)
	}
	status, err := r.services.Status(m.Resources[0], "config")
	if err != nil || status["state"] != "reserved" {
		t.Fatalf("timer started before activation: %v, %v", status, err)
	}
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	event := receiveResource(t, r)
	pending, err := r.SubmitResource(event, nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "finished" {
		t.Fatalf("timer completion: %+v", got)
	}
}

func TestRepeatingTimerStopsOnCallbackFailure(t *testing.T) {
	r := openTestRuntime(t, `wraith.timer.start(1, function() error("stop repeating") end, {repeat_ms=1})`)
	m := r.Manifest()
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	event := receiveResource(t, r)
	if event.Final || event.Kind != "tick" {
		t.Fatalf("first repeated event: %+v", event)
	}
	pending, err := r.SubmitResource(event, nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveResult(t, pending); got.Err == nil {
		t.Fatal("timer callback failure was swallowed")
	}
	status, err := r.services.Status(m.Resources[0], "config")
	if err != nil || status["state"] != "cancelled" {
		t.Fatalf("failed callback left timer running: %v, %v", status, err)
	}
	for {
		event = receiveResource(t, r)
		if event.Final {
			if event.Data["state"] != "cancelled" {
				t.Fatalf("timer final status: %+v", event)
			}
			break
		}
	}
}

func TestTimerRejectsInvalidIntervalsAndCancelsBeforeActivation(t *testing.T) {
	r := openTestRuntime(t, `for _, value in ipairs({0, -1, 0.5, 86400001}) do
 assert(not pcall(wraith.timer.start, value, function() end))
end
assert(not pcall(wraith.timer.start, 1, function() end, {repeat_ms=0}))
assert(not pcall(wraith.timer.start, 1, function() end, {repeat_ms=0/0}))
local id = wraith.timer.start(1, function() error("cancelled timer fired") end)
wraith.resource.cancel(id)`)
	m := r.Manifest()
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	if event := receiveResource(t, r); !event.Final || event.Data["state"] != "cancelled" {
		t.Fatalf("cancelled timer produced work: %+v", event)
	}
	status, err := r.services.Status(m.Resources[0], "config")
	if err != nil || status["state"] != "cancelled" || status["done"] != true {
		t.Fatalf("cancelled timer status: %v, %v", status, err)
	}
}
