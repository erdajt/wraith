package plugin

import (
	"context"
	"testing"
	"time"
)

func TestResourceActivationAndRetainedCompletion(t *testing.T) {
	m := newResourceManager(t.Context())
	t.Cleanup(m.close)
	started := make(chan struct{})
	id, err := m.reserve("config", "fixture", func(context.Context, *resourceSink) resourceResult {
		close(started)
		return resourceResult{Data: map[string]any{"bytes": []byte{0, 255}}}
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
		t.Fatal("reserved work started before activation")
	default:
	}
	if err := m.activate(id); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-m.events:
		if !event.Final || !m.valid(event) {
			t.Fatalf("terminal event lost its handle: %+v", event)
		}
		status, err := m.Status(id, "config")
		if err != nil || status["state"] != "completed" || status["done"] != true {
			t.Fatalf("status: %v, %v", status, err)
		}
		m.discard([]string{id})
		if m.valid(event) {
			t.Fatal("released event remained valid")
		}
	case <-time.After(time.Second):
		t.Fatal("completion was not delivered")
	}
}

func TestResourceCancellationUnblocksProtocolPressure(t *testing.T) {
	m := newResourceManager(t.Context())
	t.Cleanup(m.close)
	blocked := make(chan struct{})
	exited := make(chan struct{})
	id, err := m.reserve("config", "fixture", func(ctx context.Context, sink *resourceSink) resourceResult {
		defer close(exited)
		for i := 0; ; i++ {
			if i == 257 {
				close(blocked)
			}
			if err := sink.Emit("stdout", map[string]any{"data": []byte{byte(i)}}, true); err != nil {
				return resourceResult{Err: err}
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.activate(id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("worker did not reach bounded queue")
	}
	if err := m.Cancel(id, "config"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("cancel did not release protocol producer")
	}
	finals := 0
	deadline := time.After(time.Second)
	for finals == 0 {
		select {
		case event := <-m.events:
			if event.Final {
				finals++
				if event.Data["state"] != "cancelled" {
					t.Fatalf("terminal state: %+v", event)
				}
			}
		case <-deadline:
			t.Fatal("terminal event was dropped behind progress pressure")
		}
	}
}

func TestResourceReleaseCancelsReservedWork(t *testing.T) {
	m := newResourceManager(t.Context())
	t.Cleanup(m.close)
	called := make(chan struct{}, 1)
	id, err := m.reserve("plugin:example:1", "fixture", func(context.Context, *resourceSink) resourceResult { called <- struct{}{}; return resourceResult{} })
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(id, "another-owner"); err == nil {
		t.Fatal("foreign owner canceled resource")
	}
	m.discard([]string{id})
	if err := m.activate(id); err == nil {
		t.Fatal("released reservation was activated")
	}
	select {
	case <-called:
		t.Fatal("released work ran")
	default:
	}
}
