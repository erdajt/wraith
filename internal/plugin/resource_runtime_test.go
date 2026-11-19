package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResourceCallbackPreservesBinaryAndFreshWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protocol")
	if err := os.WriteFile(path, []byte{0xe2, 0x82, 0, 255, '\r', '\n'}, 0600); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, fmt.Sprintf(`wraith.fs.read_async(%q,function(ctx)
 assert(ctx.resource.final and ctx.resource.revision==1)
 assert(ctx.data.result.data==string.char(226,130,0,255,13,10))
 wraith.notify(ctx.prompt)
 end)`, path))
	m := r.Manifest()
	if len(m.Resources) != 1 {
		t.Fatalf("missing startup reservation: %+v", m.Resources)
	}
	status, err := r.services.Status(m.Resources[0], "config")
	if err != nil || status["state"] != "reserved" {
		t.Fatalf("operation started before acceptance: %v, %v", status, err)
	}
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-r.Events():
		ch, err := r.SubmitResource(event, map[string]any{"prompt": "current draft"}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		got := receiveResult(t, ch)
		if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "current draft" {
			t.Fatalf("resource callback: %+v", got)
		}
		if !r.ResourceValid(event) {
			t.Fatal("completion invalidated its retained source")
		}
		ch, err = r.SubmitResource(event, nil, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		if duplicate := receiveResult(t, ch); len(duplicate.Effects) != 0 {
			t.Fatal("resource event replayed")
		}
	case <-time.After(time.Second):
		t.Fatal("resource completion was not delivered")
	}
}

func TestFailedAndRejectedCallbacksDiscardStagedWorkers(t *testing.T) {
	for _, failure := range []string{"error", "reject"} {
		t.Run(failure, func(t *testing.T) {
			suffix := ""
			if failure == "error" {
				suffix = `error("rollback")`
			}
			r := openTestRuntime(t, fmt.Sprintf(`wraith.commands.create("start",function()
    wraith.timer.start(1,function() error("unaccepted worker ran") end)
    wraith.commands.create("staged",function() end)
    %s
   end)`, suffix))
			m := r.Manifest()
			ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
			if err != nil {
				t.Fatal(err)
			}
			got := receiveResult(t, ch)
			if failure == "reject" {
				if got.Manifest == nil {
					t.Fatalf("missing publication: %+v", got)
				}
				if err := r.Acknowledge(got.Revision, false); err != nil {
					t.Fatal(err)
				}
			} else if got.Err == nil {
				t.Fatal("expected failed callback")
			}
			r.services.mu.Lock()
			remaining := len(r.services.records)
			r.services.mu.Unlock()
			if remaining != 0 {
				t.Fatalf("unaccepted reservation leaked: %d", remaining)
			}
		})
	}
}

func TestResourceCallbackFailureCancelsContinuingWork(t *testing.T) {
	r := openTestRuntime(t, `wraith.timer.start(1,function() error("consumer failed") end,{repeat_ms=1})`)
	m := r.Manifest()
	if err := r.Activate(m.Resources); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-r.Events():
		ch, err := r.SubmitResource(event, map[string]any{}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		got := receiveResult(t, ch)
		if got.Err == nil {
			t.Fatal("consumer error disappeared")
		}
		status, err := r.services.Status(event.ID, "config")
		if err != nil || status["state"] != "cancelled" {
			encoded, _ := json.Marshal(status)
			t.Fatalf("failed consumer retained worker: %s, %v", encoded, err)
		}
	case <-time.After(time.Second):
		t.Fatal("timer did not fire")
	}
}
