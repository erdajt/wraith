package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

func TestEveryProviderRequestRedactsConversationAndPreservesCaller(t *testing.T) {
	c := config.Default()
	c.Provider = "openai"
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		if strings.Contains(string(body), "private.example") || !strings.Contains(string(body), "wraith::TARGET") {
			t.Fatalf("outbound privacy boundary failed")
		}

		return response("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"), nil
	})
	request := ollama.Request{Model: "model", Messages: []session.Message{{Role: "system", Content: "archive private.example"}, {Role: "user", Content: "check private.example"}}}

	if _, err := client.Chat(t.Context(), request, nil); err != nil {
		t.Fatal(err)
	}

	if request.Messages[0].Content != "archive private.example" {
		t.Fatal("caller request was mutated")
	}
}

func TestOllamaStartupChecksPrivateRoutingBeforeIO(t *testing.T) {
	for _, operation := range []string{"probe", "start", "load"} {
		for _, location := range []string{"base", "profile", "model"} {
			if location == "model" && operation != "load" {
				continue
			}
			t.Run(operation+"/"+location, func(t *testing.T) {
				client := New(config.Default())
				client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TOKEN", Value: "private-routing-token"}}, nil)
				model := "public-model"
				switch location {
				case "base":
					client.Base = "http://ollama.invalid/private-routing-token"
				case "profile":
					client.Profile.BaseURL = "http://ollama.invalid/private-routing-token"
				case "model":
					model = "private-routing-token"
				}
				calls := 0
				client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
					calls++
					return response(`{"models":[],"done":true}`), nil
				})

				log := filepath.Join(t.TempDir(), "server.log")
				var err error
				if operation == "load" {
					err = client.Load(t.Context(), model, "1m", ollama.Options{})
				} else {
					err = client.EnsureServer(t.Context(), operation == "start", log)
				}
				if err == nil || calls != 0 || strings.Contains(err.Error(), "private-routing-token") {
					t.Fatalf("private metadata crossed startup boundary: calls=%d, err=%v", calls, err)
				}
				if _, err := os.Stat(log); !os.IsNotExist(err) {
					t.Fatal("rejected startup created a server log")
				}
			})
		}
	}
}

func TestOllamaStartupAcceptsPublicRouting(t *testing.T) {
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	var paths []string
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		return response(`{"models":[],"done":true}`), nil
	})
	if err := client.EnsureServer(t.Context(), false, ""); err != nil {
		t.Fatal(err)
	}
	if err := client.Load(t.Context(), "public-model", "1m", ollama.Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(paths, ",") != "/api/tags,/api/generate" {
		t.Fatalf("unexpected startup requests: %v", paths)
	}
}

func TestPrivateRequestScrubsAliasedEnvironmentAndRejectsAuthCollision(t *testing.T) {
	t.Setenv("WRAITH_PRIVATE_ALIAS", "url=private.example")
	t.Setenv("WRAITH_PUBLIC_TEST", "retained")
	c := config.Default()
	c.Provider = "codex"
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	dir := t.TempDir()
	script := filepath.Join(dir, "record")
	source := "#!/bin/sh\nenv > \"$1/env\"\ncat > \"$1/input\"\nprintf '%s\\n' '{\"type\":\"turn.completed\"}'\n"
	if err := os.WriteFile(script, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}

	client.Profile.Command = []string{script, dir}

	if _, err := client.Chat(t.Context(), ollama.Request{Messages: []session.Message{{Role: "user", Content: "private.example"}}}, nil); err != nil {
		t.Fatal(err)
	}

	for _, file := range []string{"env", "input"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}

		if strings.Contains(string(data), "private.example") {
			t.Fatalf("private value reached child %s", file)
		}

		if file == "env" && !strings.Contains(string(data), "WRAITH_PUBLIC_TEST=retained") {
			t.Fatal("ordinary environment lost")
		}
	}

	t.Setenv("OPENAI_API_KEY", "private.example")

	if _, err := client.Chat(t.Context(), ollama.Request{}, nil); err == nil {
		t.Fatal("auth collision was transmitted")
	}
}

func TestInventoryNamesRemainStructuralAtProviderBoundary(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{
		{Name: "PRIMARY_TARGET_VARIABLE", Value: "private.example", Description: "check private.example"},
		{Name: "ALIAS", Value: "PRIMARY_TARGET_VARIABLE"},
	}, nil)
	request := ollama.Request{PrivateInventory: true, Messages: []session.Message{{Role: "system", Content: "check PRIMARY_TARGET_VARIABLE"}}}
	_, got, err := client.privateRequest(request)
	if err != nil {
		t.Fatal(err)
	}

	content := got.Messages[0].Content
	if !strings.Contains(content, `"name":"PRIMARY_TARGET_VARIABLE"`) || strings.Contains(content, "private.example") || !strings.HasPrefix(content, "check wraith::ALIAS") {
		t.Fatalf("incorrect inventory boundary: %s", content)
	}

	if request.Messages[0].Content != "check PRIMARY_TARGET_VARIABLE" {
		t.Fatal("caller changed")
	}

	request.PrivateInventory = false
	_, compact, err := client.privateRequest(request)
	if err != nil || strings.Contains(compact.Messages[0].Content, "Private local variables") {
		t.Fatalf("unexpected compaction inventory: %v", err)
	}
}

func TestRedactionGrowthCannotOverflowProviderContext(t *testing.T) {
	t.Parallel()
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "VERY_LONG_PRIVATE_VARIABLE_NAME", Value: "x"}}, nil)
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		t.Fatal("oversized redacted request dispatched")
		return nil, nil
	})
	_, err := client.Chat(t.Context(), ollama.Request{Messages: []session.Message{{Role: "user", Content: strings.Repeat("x ", 50)}}, Options: ollama.Options{Context: 512, Predict: 64}}, nil)
	if err == nil || !strings.Contains(err.Error(), "context budget") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStructuredEvidenceSurvivesProviderRedaction(t *testing.T) {
	client := New(config.Default())
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TOKEN", Value: "ab"}, {Name: "FIELD", Value: "line"}}, nil)
	evidence := session.Evidence{ID: strings.Repeat("ab", 12), Page: &session.OutputPage{Lines: []session.OutputLine{{Number: 1, Text: "ab line"}}}}
	request := ollama.Request{Model: "test", Messages: []session.Message{evidence.Message()}}
	_, prepared, err := client.privateRequest(request)
	if err != nil {
		t.Fatal(err)
	}

	message := prepared.Messages[0]
	if message.Evidence != nil || !strings.Contains(message.Content, evidence.ID) || !strings.Contains(message.Content, `"line":1`) || strings.Contains(message.Content, "ab line") {
		t.Fatalf("evidence boundary: %+v", message)
	}

	if request.Messages[0].Evidence == nil {
		t.Fatal("request was mutated")
	}
}

func TestPrivateValueCannotCorruptStaticExecutionSchema(t *testing.T) {
	c := config.Default()
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "FIELD", Value: "command"}}, nil)
	request := ollama.Request{ExecutionMode: "agent", Model: "test", Messages: contextual.ForChat(session.New("test"), c, "the private value is command")}
	_, prepared, err := client.privateRequest(request)
	if err != nil {
		t.Fatal(err)
	}

	content := prepared.Messages[0].Content
	if !strings.Contains(content, `{"command":"the exact shell command"`) || !strings.Contains(content, "private value is wraith::FIELD") {
		t.Fatalf("policy schema corrupted: %s", content)
	}
}

func TestCompactionSchemaSurvivesPrivateWordCollisions(t *testing.T) {
	for _, tc := range []struct{ task, policy string }{{"extract", contextual.CompactPrompt}, {"audit", contextual.AuditPrompt}} {
		t.Run(tc.task, func(t *testing.T) {
			client := New(config.Default())
			client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "FIELD", Value: "facts"}, {Name: "OTHER", Value: "missing"}}, nil)
			request := ollama.Request{CompactionTask: tc.task, Messages: []session.Message{{Role: "system", Content: tc.policy + "\nfeedback: facts missing"}, {Role: "user", Content: "facts missing"}}}
			_, got, err := client.privateRequest(request)
			if err != nil {
				t.Fatal(err)
			}

			if got.Messages[0].Content != tc.policy+"\nfeedback: wraith::FIELD wraith::OTHER" || got.Messages[1].Content != "wraith::FIELD wraith::OTHER" {
				t.Fatalf("schema or source boundary: %+v", got.Messages)
			}
		})
	}
}

func TestExecutionModeSwitchRebuildsOneSelfContainedPolicy(t *testing.T) {
	t.Parallel()
	c := config.Default()
	client := New(c)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET_DOMAIN", Value: "private.example"}}, nil)
	s := session.New("test")
	s.Messages = []session.Message{{Role: "user", Content: "inspect private.example"}}

	for _, mode := range []string{"agent", "blocks", "agent"} {
		c.Execution.Mode = mode
		request := ollama.Request{ExecutionMode: mode, PrivateInventory: true, Messages: contextual.ForChat(s, c, "")}
		_, prepared, err := client.privateRequest(request)
		if err != nil {
			t.Fatal(err)
		}

		policy, other := contextual.AgentPrompt, contextual.BlocksPrompt
		if mode == "blocks" {
			policy, other = other, policy
		}

		system := prepared.Messages[0].Content
		if strings.Count(system, policy) != 1 || strings.Contains(system, other) {
			t.Fatalf("%s policy missing, duplicated or mixed", mode)
		}

		if !strings.Contains(system, `{"command":"the exact shell command","reason":`) {
			t.Fatalf("%s request schema missing", mode)
		}

		if mode == "blocks" && strings.Contains(system, `"tool":"poll"`) {
			t.Fatal("agent polling schema leaked into blocks policy")
		}

		if !strings.Contains(system, "wraith::TARGET_DOMAIN") || strings.Contains(system, "private.example") || strings.Contains(prepared.Messages[1].Content, "private.example") {
			t.Fatal("private reference instructions or redaction lost")
		}

		if s.Messages[0].Content != "inspect private.example" {
			t.Fatal("preparation mutated the conversation")
		}
	}
}

func TestThinkingPrivacyRejectsBeforeEveryTransport(t *testing.T) {
	for _, kind := range []string{"ollama", "openai", "anthropic", "codex-cli", "claude-cli", "plugin"} {
		for _, hooked := range []bool{false, true} {
			for _, effort := range []string{"private.example", "prefix-private.example-suffix"} {
				t.Run(fmt.Sprintf("%s/hooked=%t/%s", kind, hooked, effort), func(t *testing.T) {
					client := New(config.Default())
					client.Profile = config.Provider{Kind: kind, Command: []string{"must-not-launch"}}
					client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
					client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
						t.Fatal("private thinking reached http transport")
						return nil, nil
					})
					client.Adapter = adapterFixture{chat: func(context.Context, ollama.Request, func(ollama.Response)) (ollama.Response, error) {
						t.Fatal("private thinking reached plugin adapter")
						return ollama.Response{}, nil
					}}
					request := ollama.Request{Model: "local", Think: effort}
					if hooked {
						request.Think = "high"
						client.Transform = func(_ context.Context, _ string, data PromptData) (PromptData, error) {
							data.Think = effort
							return data, nil
						}
					}

					for _, purpose := range []string{"", "extract", "audit"} {
						request.CompactionTask = purpose
						_, err := client.ChatWithRepair(t.Context(), request, nil)
						if err == nil || !strings.Contains(err.Error(), "thinking metadata") || strings.Contains(err.Error(), "private.example") {
							t.Fatalf("private thinking was not rejected safely: %v", err)
						}
					}
				})
			}
		}
	}
}

func TestThinkingPrivacyPreservesPublicValues(t *testing.T) {
	for _, effort := range []any{nil, false, true, "", "high", "custom-effort"} {
		t.Run(fmt.Sprintf("%T/%v", effort, effort), func(t *testing.T) {
			client := New(config.Default())
			client.Profile.Kind = "plugin"
			client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
			calls := 0
			client.Adapter = adapterFixture{chat: func(_ context.Context, request ollama.Request, _ func(ollama.Response)) (ollama.Response, error) {
				calls++
				if request.Think != effort {
					t.Fatalf("thinking changed: got %#v, want %#v", request.Think, effort)
				}
				return ollama.Response{Done: true}, nil
			}}
			if _, err := client.Chat(t.Context(), ollama.Request{Model: "local", Think: effort}, nil); err != nil || calls != 1 {
				t.Fatalf("public thinking rejected: calls=%d error=%v", calls, err)
			}
		})
	}
}
