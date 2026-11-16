package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestRepairStreamsReplacementsAndExplainsRejection(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	calls, retries := 0, 0
	var streamed strings.Builder
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "private.example") {
			t.Fatal("private value transmitted")
		}
		if calls > 1 && !strings.Contains(string(body), "unknown variable MISSING") {
			t.Fatal("repair omitted failure")
		}
		chunks := []string{"checking ", "wraith::MIS", "SING "}
		if calls > 1 {
			chunks = []string{"inspect ", "wraith:", ":TARGET", " now"}
		}
		var out strings.Builder
		for _, text := range chunks {
			data, _ := json.Marshal(ollama.Response{Message: session.Message{Role: "assistant", Content: text}})
			fmt.Fprintln(&out, string(data))
		}
		fmt.Fprintln(&out, `{"done":true,"message":{"role":"assistant"}}`)
		return response(out.String()), nil
	})
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{PrivateInventory: true}, func(update StreamUpdate) {
		if update.Retry > 0 {
			retries++
			streamed.Reset()
			return
		}
		streamed.WriteString(update.Chunk.Message.Content)
		if strings.Contains(streamed.String(), "wraith::MIS") {
			t.Fatal("incomplete marker emitted")
		}
	})
	if err != nil || calls != 2 || retries != 1 || result.Message.Content != "inspect wraith::TARGET now" || streamed.String() != result.Message.Content {
		t.Fatalf("repair failed: calls=%d retries=%d result=%q streamed=%q err=%v", calls, retries, result.Message.Content, streamed.String(), err)
	}
}

func TestRepairStopsAfterThreeRetriesAndRetainsInvalidAnswer(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	calls := 0
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		calls++
		return response("{\"message\":{\"role\":\"assistant\",\"content\":\"wraith::MISSING \"}}\n{\"done\":true}\n"), nil
	})
	result, err := client.ChatWithRepair(t.Context(), ollama.Request{}, nil)
	if calls != 4 || err == nil || !strings.Contains(err.Error(), "3 retries") || !strings.Contains(result.Message.Content, "wraith::MISSING") || !result.Message.Interrupted {
		t.Fatalf("unexpected exhaustion: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestRepairDoesNotRetryTransportFailuresOrOverflow(t *testing.T) {
	t.Parallel()
	for _, transportFailure := range []bool{true, false} {
		client := New(config.Default())
		calls := 0
		client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
			calls++
			if transportFailure {
				return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
			}
			return response("{\"message\":{\"content\":\"wraith::MISSING \"}}\n{\"done\":true}\n"), nil
		})
		_, err := client.ChatWithRepair(t.Context(), ollama.Request{Options: ollama.Options{Context: 240, Predict: 64}}, nil)
		if err == nil || calls != 1 {
			t.Fatalf("unexpected retry: calls=%d err=%v", calls, err)
		}
	}
}
