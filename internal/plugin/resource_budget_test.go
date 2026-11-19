package plugin

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestResourceProgressByteBudgetAndCoalescing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	m := &resourceManager{
		ctx: ctx, cancel: cancel, records: map[string]*managedResource{},
		changed: make(chan struct{}), wake: make(chan struct{}, 1),
	}
	for _, id := range []string{"a", "b", "c"} {
		m.records[id] = &managedResource{id: id, owner: "config", state: "running", cancel: cancel}
	}
	a := &resourceSink{ID: "a", Owner: "config", manager: m, ctx: ctx}
	b := &resourceSink{ID: "b", Owner: "config", manager: m, ctx: ctx}
	c := &resourceSink{ID: "c", Owner: "config", manager: m, ctx: ctx}
	payload := map[string]any{"data": strings.Repeat("x", 3<<20)}
	for _, sink := range []*resourceSink{a, b, c} {
		if err := sink.Emit("capture", payload, false); err != nil {
			t.Fatal(err)
		}
	}
	if m.progress != 2 || m.records["c"].dropped != 1 || m.progressBytes > 8<<20 {
		t.Fatalf("capture budget: count=%d bytes=%d dropped=%d", m.progress, m.progressBytes, m.records["c"].dropped)
	}
	if err := a.Emit("capture", map[string]any{"data": strings.Repeat("x", 1<<20)}, false); err != nil {
		t.Fatal(err)
	}
	if err := c.Emit("capture", payload, false); err != nil {
		t.Fatal(err)
	}
	if m.progress != 3 || m.progressBytes > 8<<20 {
		t.Fatalf("coalesced budget: count=%d bytes=%d", m.progress, m.progressBytes)
	}

	m.terminal(m.records["b"])
	completed := make(chan error, 1)
	go func() { completed <- a.Emit("protocol", map[string]any{"data": strings.Repeat("x", 2<<20)}, true) }()
	select {
	case err := <-completed:
		t.Fatalf("protocol bypassed byte pressure: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	m.mu.Lock()
	m.removeQueued([]string{"b"})
	m.mu.Unlock()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("removing queued data did not release protocol producer")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.progress != 3 || m.progressBytes > 8<<20 {
		t.Fatalf("protocol budget: count=%d bytes=%d", m.progress, m.progressBytes)
	}
	m.removeQueued([]string{"a", "c"})
	if m.progress != 0 || m.progressBytes != 0 || len(m.queue) != 0 {
		t.Fatalf("released queue retained byte charge: %d", m.progressBytes)
	}
}

func TestResourceProgressRejectsSingleEventBeyondByteBudget(t *testing.T) {
	sink := &resourceSink{ID: "fixture"}
	err := sink.Emit("protocol", map[string]any{"data": strings.Repeat("x", (8<<20)-4)}, true)
	if err == nil || !strings.Contains(err.Error(), "progress event exceeds") {
		t.Fatalf("oversized event did not fail before waiting: %v", err)
	}
}
