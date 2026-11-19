package plugin

import (
	"fmt"
	"testing"
)

func TestScheduledSiblingsSurvivePublicationAndFailure(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("queue",function()
 wraith.schedule(function() wraith.commands.create("child",function() end) end)
 wraith.schedule(function() error("scheduled failure") end)
 wraith.schedule(function() wraith.notify("last") end)
 end)`)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Err != nil || len(got.Effects) != 3 {
		t.Fatalf("scheduled batch: %+v", got)
	}
	var queued []<-chan Result
	for _, effect := range got.Effects {
		if effect.Name != "schedule.callback" {
			t.Fatalf("unexpected effect: %+v", effect)
		}
		ch, err = r.Submit(effect.Argument, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		queued = append(queued, ch)
	}
	first := receiveResult(t, queued[0])
	if first.Err != nil || first.Manifest == nil {
		t.Fatalf("first callback: %+v", first)
	}
	if err := r.Acknowledge(first.Revision, true); err != nil {
		t.Fatal(err)
	}
	if second := receiveResult(t, queued[1]); second.Err == nil {
		t.Fatal("scheduled failure was swallowed")
	}
	if third := receiveResult(t, queued[2]); third.Err != nil || len(third.Effects) != 1 || third.Effects[0].Argument != "last" {
		t.Fatalf("later sibling was lost: %+v", third)
	}
	for _, effect := range got.Effects {
		ch, err = r.Submit(effect.Argument, "", nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		if replay := receiveResult(t, ch); replay.Err != nil || len(replay.Effects) != 0 || replay.Manifest != nil {
			t.Fatalf("scheduled work replayed: %+v", replay)
		}
	}
}

func TestDiscardedScheduledWorkReleasesPendingCapacity(t *testing.T) {
	r := openTestRuntime(t, `wraith.commands.create("queue",function()
 for i=1,64 do wraith.schedule(function() error("discarded callback ran") end) end
 end)`)
	for range 5 {
		m := r.Manifest()
		ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		got := receiveResult(t, ch)
		if got.Err != nil || len(got.Effects) != 64 {
			t.Fatalf("pending slots leaked: %+v", got)
		}
		var ids []string
		for _, effect := range got.Effects {
			ids = append(ids, effect.Argument)
		}
		r.DiscardScheduled(ids)
		ch, err = r.Submit(ids[0], "", nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		if skipped := receiveResult(t, ch); skipped.Err != nil || len(skipped.Effects) != 0 {
			t.Fatalf("discarded work dispatched: %+v", skipped)
		}
	}
}

func TestStartupScheduleBoundaryAndUnloadedOwners(t *testing.T) {
	for _, count := range []int{1, 64} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			r := openTestRuntime(t, fmt.Sprintf(`for i=1,%d do wraith.schedule(function() wraith.notify("ready") end) end
   assert(not pcall(function() for i=1,65 do wraith.schedule(function() end) end end))`, count))
			m := r.Manifest()
			ch, err := r.Submit("", "ready", nil, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			got := receiveResult(t, ch)
			if got.Err != nil || got.Manifest == nil || len(got.Effects) != 64 {
				t.Fatalf("startup schedules: %+v", got)
			}
			if err := r.Acknowledge(got.Revision, true); err != nil {
				t.Fatal(err)
			}
		})
	}
	r := openTestRuntime(t, `package.preload.later=function() return {setup=function()
 wraith.schedule(function() error("unloaded owner ran") end)
 end} end
 wraith.plugins.load("later")
 wraith.plugins.unload("later")`)
	m := r.Manifest()
	ch, err := r.Submit("", "ready", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Err != nil || len(got.Effects) != 0 {
		t.Fatalf("unloaded startup work remained: %+v", got)
	}
	if got.Manifest != nil {
		if err := r.Acknowledge(got.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStartupSchedulingDoesNotConsumeObserverActionBudget(t *testing.T) {
	for _, first := range []bool{false, true} {
		t.Run(fmt.Sprint(first), func(t *testing.T) {
			observer := `wraith.events.on("ready",function() for i=1,64 do wraith.notify("ready") end end)`
			scheduled := `for i=1,64 do wraith.schedule(function() end) end`
			source := observer + "\n" + scheduled
			if !first {
				source = scheduled + "\n" + observer
			}
			r := openTestRuntime(t, source)
			ch, err := r.Submit("", "ready", nil, r.Manifest().Config)
			if err != nil {
				t.Fatal(err)
			}
			got := receiveResult(t, ch)
			if got.Err != nil || len(got.Effects) != 128 || got.Manifest == nil {
				t.Fatalf("mixed startup batch: %+v", got)
			}
			if err := r.Acknowledge(got.Revision, true); err != nil {
				t.Fatal(err)
			}
			if len(r.Diagnostics()) != 0 {
				t.Fatalf("startup observer failed: %v", r.Diagnostics())
			}
		})
	}
}
