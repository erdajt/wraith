package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func TestCLIInvocationUsesIsolatedConversationAndRestrictedTools(t *testing.T) {
	for _, kind := range []string{"codex-cli", "claude-cli"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			script := filepath.Join(dir, "record")
			source := `#!/bin/sh
out="$1"
shift
printf '%s\n' "$PWD" > "$out/cwd"
printf '%s\0' "$@" > "$out/args"
cat > "$out/input"
previous=''
for arg do
  if [ "$previous" = '--system-prompt-file' ]; then
    cat "$arg" > "$out/system"
    stat -c '%a' "$arg" > "$out/mode"
  fi
  previous="$arg"
done
printf '%s\n' '{"type":"turn.completed"}' '{"type":"result","result":"ok"}'
`
			if err := os.WriteFile(script, []byte(source), 0700); err != nil {
				t.Fatal(err)
			}
			req := ollama.Request{Model: "chosen-model", Think: "high", Messages: []session.Message{{Role: "system", Content: "custom instructions"}, {Role: "user", Content: "literal $(no execution)"}}}
			if _, err := CLI(context.Background(), config.Provider{Kind: kind, Command: []string{script, dir}}, req, nil); err != nil {
				t.Fatal(err)
			}
			read := func(name string) string {
				t.Helper()
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			cwd := strings.TrimSpace(read("cwd"))
			if cwd == dir || !strings.HasPrefix(filepath.Base(cwd), "wraith-provider-") {
				t.Fatal("cli did not use isolated cwd")
			}
			if _, err := os.Stat(cwd); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("temporary context remained on disk")
			}
			args := strings.Split(strings.TrimSuffix(read("args"), "\x00"), "\x00")
			pair := func(flag, value string) {
				t.Helper()
				index := slices.Index(args, flag)
				if index < 0 || index+1 >= len(args) || args[index+1] != value {
					t.Fatalf("missing %s %q", flag, value)
				}
			}
			pair("--model", "chosen-model")
			if kind == "codex-cli" {
				pair("--sandbox", "read-only")
				pair("--disable", "shell_tool")
				for _, arg := range []string{"--ignore-user-config", "--ephemeral", "--json", `developer_instructions="custom instructions"`, `model_reasoning_effort="high"`, `web_search="disabled"`, "apps._default.enabled=false"} {
					if !slices.Contains(args, arg) {
						t.Fatalf("missing %s", arg)
					}
				}
			} else {
				pair("--tools", "")
				pair("--disallowedTools", "mcp__*")
				pair("--effort", "high")
				pair("--output-format", "stream-json")
				if !slices.Contains(args, "--safe-mode") || read("system") != "custom instructions" || strings.TrimSpace(read("mode")) != "600" {
					t.Fatal("system context or restrictions missing")
				}
			}
			_, payload, ok := strings.Cut(read("input"), "Conversation JSON:\n")
			var messages []session.Message
			if !ok || json.Unmarshal([]byte(payload), &messages) != nil || len(messages) != 1 || messages[0].Content != req.Messages[1].Content {
				t.Fatal("conversation input was altered")
			}
		})
	}
}

func TestCLIRejectsNativeToolsAtStart(t *testing.T) {
	for _, test := range []struct{ kind, event string }{
		{"codex-cli", `{"type":"item.started","item":{"type":"command_execution"}}`},
		{"claude-cli", `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"tool_use"}}}`},
	} {
		if err := cliEvent(test.kind, []byte(test.event), &ollama.Response{}, nil); err == nil {
			t.Fatal("native tool event ignored")
		}
	}
}
