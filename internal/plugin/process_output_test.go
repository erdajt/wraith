package plugin

import (
	"bytes"
	"context"
	"testing"
)

func TestProcessCaptureRetainsExactByteWindows(t *testing.T) {
	manager := newResourceManager(t.Context())
	t.Cleanup(manager.close)
	var output *processOutput
	id, err := manager.reserve("config", "process", func(_ context.Context, sink *resourceSink) resourceResult {
		output = &processOutput{limit: 5, stream: "stdout", sink: sink}
		for _, part := range [][]byte{{'a', 0, 0xff}, {0xe7, 0x95}, {0x8c, '\r', '\n'}} {
			if _, err := output.Write(part); err != nil {
				return resourceResult{Err: err}
			}
		}
		return resourceResult{Data: map[string]any{}}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.activate(id); err != nil {
		t.Fatal(err)
	}
	for event := range manager.events {
		if event.Final {
			break
		}
		if _, ok := event.Data["data"]; ok {
			t.Fatal("capture notification duplicated retained bytes")
		}
	}
	window, err := output.query(0, 3)
	if err != nil || !bytes.Equal(window["data"].([]byte), []byte{0xe7, 0x95, 0x8c}) || window["truncated"] != true || window["offset"] != int64(3) || window["next_offset"] != int64(6) {
		t.Fatalf("retained window: %v, %v", window, err)
	}
	window["data"].([]byte)[0] = 42
	last, err := output.query(3, 10)
	if err != nil || !bytes.Equal(last["data"].([]byte), []byte{0xe7, 0x95, 0x8c, '\r', '\n'}) {
		t.Fatalf("query aliased retained output: %v, %v", last, err)
	}
}

func TestProcessProtocolPreservesEveryChunk(t *testing.T) {
	manager := newResourceManager(t.Context())
	t.Cleanup(manager.close)
	want := bytes.Repeat([]byte{0, 0xff, 0xe7, 0x95, 0x8c, '\r', '\n'}, 20000)
	id, err := manager.reserve("config", "process", func(_ context.Context, sink *resourceSink) resourceResult {
		output := &processOutput{limit: 64, stream: "stderr", protocol: true, sink: sink}
		_, err := output.Write(want)
		return resourceResult{Data: map[string]any{}, Err: err}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.activate(id); err != nil {
		t.Fatal(err)
	}
	var got []byte
	for event := range manager.events {
		if event.Final {
			break
		}
		if event.Data["offset"] != float64(len(got)) {
			t.Fatalf("protocol offset skipped bytes: %v", event.Data)
		}
		got = append(got, event.Data["data"].([]byte)...)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("protocol transport changed binary data")
	}
}
