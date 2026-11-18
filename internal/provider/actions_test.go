package provider

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
)

func TestActionTransportSelectionAndPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		kind, protocol, mode string
		capabilities         []string
		native, invalid      bool
	}{
		{"ollama", "auto", "agent", []string{"tools"}, true, false},
		{"ollama", "auto", "agent", nil, false, false},
		{"ollama", "text", "agent", []string{"tools"}, false, false},
		{"openai", "auto", "agent", nil, false, false},
		{"openai", "native", "agent", nil, true, false},
		{"anthropic", "native", "blocks", nil, true, false},
		{"codex-cli", "auto", "agent", nil, false, false},
		{"claude-cli", "native", "agent", nil, false, true},
	} {
		t.Run(tc.kind+"/"+tc.protocol+"/"+tc.mode, func(t *testing.T) {
			t.Parallel()
			c := config.Default()
			p := c.Providers[c.Provider]
			p.Kind, p.ActionProtocol = tc.kind, tc.protocol
			c.Providers[c.Provider], c.Execution.Mode = p, tc.mode
			client := New(c)
			_, prepared, err := client.privateRequest(ollama.Request{Capabilities: tc.capabilities, ExecutionMode: tc.mode, Messages: contextual.ForChat(session.New("test"), c, "")})
			if (err != nil) != tc.invalid {
				t.Fatalf("unexpected protocol error: %v", err)
			}
			if tc.invalid {
				return
			}
			if (len(prepared.Tools) > 0) != tc.native {
				t.Fatal("wrong tool transport")
			}
			policy := prepared.Messages[0].Content
			if tc.native && (strings.Contains(policy, "fenced wraith-run") || strings.Contains(policy, "fenced wraith-tool")) {
				t.Fatal("native policy retained text protocol instructions")
			}
			if tc.native && tc.mode == "blocks" && len(prepared.Tools) != 1 {
				t.Fatal("blocks mode offered output tools")
			}
		})
	}
}

func TestCompactionAndStructuredRequestsExcludeTools(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	client.Profile.ActionProtocol = "native"
	for _, request := range []ollama.Request{{ExecutionMode: "agent", CompactionTask: "extract"}, {ExecutionMode: "agent", Format: "json"}, {}} {
		_, prepared, err := client.privateRequest(request)
		if err != nil || len(prepared.Tools) != 0 {
			t.Fatalf("tools leaked into a non-chat request: %v", err)
		}
	}
}

func TestRequestBudgetIncludesToolsAndCannotOverflow(t *testing.T) {
	t.Parallel()
	request := ollama.Request{Options: ollama.Options{Context: 300, Predict: 64}}
	if !requestFits(request) {
		t.Fatal("empty request should fit")
	}
	request.Tools = contextual.ActionTools("agent")
	if requestFits(request) {
		t.Fatal("schemas were not budgeted")
	}
	request.Options.Context = int(^uint(0) >> 1)
	request.Options.Predict = request.Options.Context
	if requestFits(request) {
		t.Fatal("output reservation overflowed")
	}
}
