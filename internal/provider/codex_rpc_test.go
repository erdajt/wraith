package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexRPCInitializesAndPaginatesWithoutStartingATurn(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "codex")
	source := `#!/bin/sh
out="$1"
shift
printf '%s\n' "$@" > "$out/args"
IFS= read -r line
printf '%s\n' "$line" >> "$out/requests"
printf '%s\n' '{"id":1,"result":{"userAgent":"test"}}'
IFS= read -r line
printf '%s\n' "$line" >> "$out/requests"
IFS= read -r line
printf '%s\n' "$line" >> "$out/requests"
printf '%s\n' '{"method":"notification"}' '{"id":2,"result":{"data":[{"model":"first","supportedReasoningEfforts":[{"reasoningEffort":"high"}]},{"model":"hidden","hidden":true}],"nextCursor":"page2"}}'
IFS= read -r line
printf '%s\n' "$line" >> "$out/requests"
printf '%s\n' '{"id":3,"result":{"data":[{"model":"second"},{"model":"first"}],"nextCursor":null}}'
cat >/dev/null
`
	if err := os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	models, err := codexRPC(t.Context(), []string{script, dir}, nil)
	if err != nil || len(models) != 2 || models[0].Name != "first" || models[1].Name != "second" || models[0].Efforts[0] != "high" {
		t.Fatalf("models: %+v, %v", models, err)
	}
	args, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil || string(args) != "app-server\n--listen\nstdio://\n" {
		t.Fatalf("unexpected invocation: %s, %v", args, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "requests"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("requests: %s", data)
	}
	for i, method := range []string{"initialize", "initialized", "model/list", "model/list"} {
		var request struct {
			Method string `json:"method"`
			Params struct {
				Cursor string `json:"cursor"`
			} `json:"params"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &request); err != nil || request.Method != method {
			t.Fatalf("request %d: %s", i, lines[i])
		}
		if i == 3 && request.Params.Cursor != "page2" {
			t.Fatal("pagination cursor was not forwarded")
		}
	}
}

func TestCodexRPCStopsAnUnresponsiveChild(t *testing.T) {
	script := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := codexRPC(ctx, []string{script}, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
}
