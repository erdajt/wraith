package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

func TestInvocationStreamsBeforeTerminalAndReleasesQueueStorage(t *testing.T) {
	manager := newInvocationManager("test", func([]string) {})
	t.Cleanup(manager.close)
	invocation, err := manager.begin(t.Context(), InvocationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	defer invocation.Close()
	for _, effect := range []InvocationEffect{
		{ID: invocation.ID, Kind: "emit", Data: json.RawMessage(`"one"`)},
		{ID: invocation.ID, Kind: "emit", Data: json.RawMessage(`"two"`)},
		{ID: invocation.ID, Kind: "finish", Data: json.RawMessage(`{"done":true}`)},
	} {
		if err := manager.commit(effect); err != nil {
			t.Fatal(err)
		}
	}
	for index, want := range []string{`"one"`, `"two"`, `{"done":true}`} {
		event, err := invocation.Next(t.Context())
		if err != nil || string(event.Data) != want || event.Sequence != uint64(index+1) || event.Final != (index == 2) {
			t.Fatalf("unordered stream: %+v %v", event, err)
		}
	}
	if _, err := invocation.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatalf("terminal repeated: %v", err)
	}
	manager.mu.Lock()
	bytes := manager.records[invocation.ID].bytes
	manager.mu.Unlock()
	if bytes != 0 {
		t.Fatal("consumed bytes retained their queue charge")
	}
	if err := manager.commit(InvocationEffect{ID: invocation.ID, Kind: "emit"}); err == nil {
		t.Fatal("completed invocation accepted another delta")
	}
}

func TestInvocationOverflowKeepsTerminalErrorAndCancelsChildren(t *testing.T) {
	var lock sync.Mutex
	var canceled []string
	manager := newInvocationManager("test", func(ids []string) { lock.Lock(); canceled = append(canceled, ids...); lock.Unlock() })
	t.Cleanup(manager.close)
	invocation, err := manager.begin(t.Context(), InvocationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	defer invocation.Close()
	if err := manager.attach(invocation.ID, "child"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 64; i++ {
		if err := manager.commit(InvocationEffect{ID: invocation.ID, Kind: "emit"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.commit(InvocationEffect{ID: invocation.ID, Kind: "emit"}); err == nil {
		t.Fatal("unbounded invocation queue")
	}
	for i := 0; i < 64; i++ {
		if _, err := invocation.Next(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	event, err := invocation.Next(t.Context())
	if err != nil || !event.Final || event.Error == "" {
		t.Fatalf("overflow lost terminal failure: %+v %v", event, err)
	}
	lock.Lock()
	defer lock.Unlock()
	if len(canceled) != 1 || canceled[0] != "child" {
		t.Fatalf("overflow retained child workers: %v", canceled)
	}
}

func TestInvocationDeadlineAndCloseReleaseCapacity(t *testing.T) {
	manager := newInvocationManager("test", func([]string) {})
	t.Cleanup(manager.close)
	invocation, err := manager.begin(t.Context(), InvocationRequest{Timeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	event, err := invocation.Next(ctx)
	if err != nil || !event.Final || event.Error == "" {
		t.Fatalf("missing deadline: %+v %v", event, err)
	}
	invocation.Close()
	manager.mu.Lock()
	count := len(manager.records)
	manager.mu.Unlock()
	if count != 0 {
		t.Fatal("closed invocation retained its terminal payload")
	}
	for i := 0; i < 32; i++ {
		handle, err := manager.begin(t.Context(), InvocationRequest{})
		if err != nil {
			t.Fatal(err)
		}
		defer handle.Close()
	}
	if _, err := manager.begin(t.Context(), InvocationRequest{}); err == nil {
		t.Fatal("live invocation bound was not enforced")
	}
}

func TestClosedUnstartedInvocationsDoNotOccupyDispatchCapacity(t *testing.T) {
	manager := newInvocationManager("test", func([]string) {})
	t.Cleanup(manager.close)
	for i := 0; i < 512; i++ {
		call, err := manager.begin(t.Context(), InvocationRequest{})
		if err != nil {
			t.Fatalf("closed call %d retained a dispatch slot: %v", i, err)
		}
		call.Close()
	}
	if len(manager.starts) != 0 {
		t.Fatal("closed invocations retained dispatch notices")
	}
	first, err := manager.begin(t.Context(), InvocationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := manager.begin(t.Context(), InvocationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	last, err := manager.begin(t.Context(), InvocationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	removed.Close()
	if a, b := <-manager.starts, <-manager.starts; a.ID != first.ID || b.ID != last.ID {
		t.Fatalf("dispatch order changed: %s, %s", a.ID, b.ID)
	}
}
