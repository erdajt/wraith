package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestCLIAdaptersReadNativeEventsWithoutDuplicateText(t *testing.T) {
	for _, tt := range []struct{ kind, events string }{
		{"codex-cli", `{"type":"item.completed","item":{"type":"reasoning","text":"check"}}
{"type":"item.completed","item":{"type":"agent_message","text":"answer"}}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":2}}`},
		{"claude-cli", `{"type":"stream_event","event":{"delta":{"thinking":"check"}}}
{"type":"stream_event","event":{"delta":{"text":"answer"}}}
{"type":"result","result":"answer","usage":{"input_tokens":10,"output_tokens":2}}`},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			script := filepath.Join(t.TempDir(), "cli")
			if err := os.WriteFile(script, []byte("#!/bin/sh\ncat >/dev/null\ncat <<'EVENTS'\n"+tt.events+"\nEVENTS\n"), 0700); err != nil {
				t.Fatal(err)
			}
			result, err := CLI(context.Background(), config.Provider{Kind: tt.kind, Command: []string{script}}, ollama.Request{Model: "default", Messages: []session.Message{{Role: "user", Content: "literal $(not executed)"}}}, nil)
			if err != nil || !result.Done || result.Message.Content != "answer" || result.Message.Thinking != "check" || result.PromptCount != 10 {
				t.Fatalf("result %+v %v", result, err)
			}
		})
	}
}

func TestCLICancellationStopsTheProcessGroup(t *testing.T) {
	script := filepath.Join(t.TempDir(), "cli")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat >/dev/null\nsleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := CLI(ctx, config.Provider{Kind: "codex-cli", Command: []string{script}}, ollama.Request{}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestCLIRejectsToolExecutionAndReportedFailures(t *testing.T) {
	for _, event := range []string{`{"type":"item.completed","item":{"type":"command_execution"}}`, `{"type":"turn.failed"}`} {
		if err := cliEvent("codex-cli", []byte(event), &ollama.Response{}, nil); err == nil {
			t.Fatal("native failure accepted")
		}
	}
	err := cliEvent("claude-cli", []byte(`{"type":"result","is_error":true,"result":"secret"}`), &ollama.Response{}, nil)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("cli failure exposed private output")
	}
}
