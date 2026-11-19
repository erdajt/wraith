package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

type adapterFixture struct {
	chat func(context.Context, ollama.Request, func(ollama.Response)) (ollama.Response, error)
}

func (adapterFixture) Models(context.Context) ([]ollama.Model, error) {
	return []ollama.Model{{Name: "local"}}, nil
}

func (adapterFixture) Show(context.Context, string) (ollama.Info, error) {
	return ollama.Info{ContextWindow: 8192}, nil
}

func (a adapterFixture) Chat(ctx context.Context, request ollama.Request, emit func(ollama.Response)) (ollama.Response, error) {
	return a.chat(ctx, request, emit)
}

func TestExtensionRequestIsFrozenBeforeRepairAndRedactedAfterHook(t *testing.T) {
	c := New(config.Default())
	c.Profile.Kind = "plugin"
	c.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	hooks, calls := 0, 0
	c.Transform = func(_ context.Context, purpose string, data PromptData) (PromptData, error) {
		hooks++
		if purpose != "chat" || strings.Contains(data.Messages[1].Content, "secret output") || data.Messages[1].Reference == "" {
			t.Fatal("hook purpose or protected evidence changed")
		}
		data.Messages[0].Content += " private.example transformed"
		return data, nil
	}
	c.Adapter = adapterFixture{chat: func(_ context.Context, request ollama.Request, _ func(ollama.Response)) (ollama.Response, error) {
		calls++
		var content strings.Builder
		for _, message := range request.Messages {
			content.WriteString(message.Content)
		}
		text := content.String()
		if strings.Contains(text, "private.example") || strings.Contains(text, "secret output") || strings.Count(text, "transformed") != 1 || !strings.Contains(text, contextual.AgentPrompt) {
			t.Fatal("transformed request bypassed privacy, evidence or execution policy")
		}
		if calls == 1 {
			return ollama.Response{Done: true, Message: session.Message{Content: "wraith::MISSING"}}, nil
		}
		if !strings.Contains(text, "previous answer was rejected") {
			t.Fatal("hook erased corrective feedback")
		}
		return ollama.Response{Done: true, Message: session.Message{Content: "ready"}}, nil
	}}
	request := ollama.Request{ExecutionMode: "agent", Model: "local", Messages: []session.Message{
		{Role: "user", Content: "inspect"},
		(session.Evidence{ID: "0123456789abcdef01234567", Hidden: true, Output: "secret output"}).Message(),
	}}
	if _, err := c.Estimate(request); err != nil || hooks != 0 {
		t.Fatalf("estimate invoked a hook: count=%d error=%v", hooks, err)
	}
	if _, err := c.ChatWithRepair(t.Context(), request, nil); err != nil {
		t.Fatal(err)
	}
	if hooks != 1 || calls != 2 || request.Messages[0].Content != "inspect" {
		t.Fatalf("request mutation or retry count: hooks=%d calls=%d", hooks, calls)
	}
}

func TestExtensionHookFailuresStopBeforeTransport(t *testing.T) {
	for _, failure := range []string{"duplicate", "content", "role", "unknown", "error", "budget"} {
		t.Run(failure, func(t *testing.T) {
			c := New(config.Default())
			c.Profile.Kind = "plugin"
			c.Adapter = adapterFixture{chat: func(context.Context, ollama.Request, func(ollama.Response)) (ollama.Response, error) {
				t.Fatal("failed transform reached transport")
				return ollama.Response{}, nil
			}}
			c.Transform = func(_ context.Context, _ string, data PromptData) (PromptData, error) {
				switch failure {
				case "duplicate":
					data.Messages = append(data.Messages, data.Messages[0])
				case "content":
					data.Messages[0].Content = "replacement"
				case "role":
					data.Messages[0].Role = "system"
				case "unknown":
					data.Messages[0].Reference = "other"
				case "error":
					return data, errors.New("denied")
				case "budget":
					data.Options.Context = 128
				}
				return data, nil
			}
			request := ollama.Request{Model: "local", Messages: []session.Message{(session.Evidence{Hidden: true}).Message()}}
			_, err := c.Chat(t.Context(), request, nil)
			if err == nil {
				t.Fatal("invalid transformation accepted")
			}
			var overflow *ContextError
			if failure == "budget" && !errors.As(err, &overflow) {
				t.Fatalf("budget failure lost its recovery type: %v", err)
			}
		})
	}
}

func TestPluginProviderDiscoveryRequiresRegisteredTransport(t *testing.T) {
	cfg := config.Default()
	cfg.Providers["local"] = config.Provider{Kind: "plugin", Model: "local"}
	cfg.Provider = "local"
	if err := cfg.ValidateProviders(); err != nil {
		t.Fatal(err)
	}
	c := New(cfg)
	if _, err := c.Models(t.Context()); err == nil {
		t.Fatal("missing adapter silently fell back to another provider")
	}
	c.Adapter = adapterFixture{}
	models, err := c.Models(t.Context())
	if err != nil || len(models) != 1 || models[0].Name != "local" {
		t.Fatalf("models=%v error=%v", models, err)
	}
	info, err := c.Show(t.Context(), "local")
	if err != nil || info.ContextWindow != 8192 {
		t.Fatalf("info=%v error=%v", info, err)
	}
}
