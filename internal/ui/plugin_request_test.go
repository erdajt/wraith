package ui

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/variables"
)

func TestRequestHooksCaptureIdentityAndFollowProviderRebuilds(t *testing.T) {
	a := pluginFixture(t, `
wraith.hooks.add("before_request", function(ctx)
  ctx.request.messages[1].content = ctx.session_id .. ":" .. ctx.directory .. ":" .. ctx.request.messages[1].content
  return {request=ctx.request}
end)`)
	client := a.providerClient(a.Config)
	id, directory := a.Session.ID, a.Directory
	a.Session.ID, a.Directory = "new-session", "/new-directory"
	data := provider.PromptData{Model: "local", Messages: []provider.PromptMessage{{Role: "user", Content: "inspect"}}}
	transformed, err := client.Transform(t.Context(), "chat", data)
	if err != nil || transformed.Messages[0].Content != id+":"+directory+":inspect" {
		t.Fatalf("worker read mutable workspace identity: %+v, %v", transformed, err)
	}
	if data.Messages[0].Content != "inspect" {
		t.Fatal("hook changed caller-owned message storage")
	}

	rebuilt := a.providerClient(a.Config)
	transformed, err = rebuilt.Transform(t.Context(), "chat", data)
	if err != nil || transformed.Messages[0].Content != "new-session:/new-directory:inspect" {
		t.Fatalf("rebuilt provider retained previous identity: %+v, %v", transformed, err)
	}
}

func TestRequestHooksAreOptInForCompactionAndCanDeny(t *testing.T) {
	a := pluginFixture(t, `
wraith.hooks.add("before_request", function(ctx)
  return {decision="deny",reason="scope is missing"}
end)`)
	client := a.providerClient(a.Config)
	data := provider.PromptData{Model: "local", Messages: []provider.PromptMessage{{Role: "user", Content: "inspect"}}}
	if _, err := client.Transform(t.Context(), "chat", data); err == nil || !strings.Contains(err.Error(), "scope is missing") {
		t.Fatalf("hook denial lost: %v", err)
	}
	if _, err := client.Transform(t.Context(), "extract", data); err != nil {
		t.Fatalf("chat hook intercepted compaction: %v", err)
	}
}

func TestDynamicRequestHookPublicationRefreshesFutureClients(t *testing.T) {
	a := pluginFixture(t, `
wraith.commands.create("installhook", function()
  wraith.hooks.add("before_request", function(ctx)
    ctx.request.messages[1].content = "transformed"
    return {request=ctx.request}
  end)
end)`)
	a.Client = a.providerClient(a.Config)
	if a.Client.Transform != nil {
		t.Fatal("empty hook registry imposed a request callback")
	}
	applyPluginEvents(t, a, a.runCommand("installhook"))
	if a.Client.Transform == nil {
		t.Fatal("accepted registry did not update future requests")
	}
	data := provider.PromptData{Model: "local", Messages: []provider.PromptMessage{{Role: "user", Content: "inspect"}}}
	result, err := a.Client.Transform(t.Context(), "chat", data)
	if err != nil || result.Messages[0].Content != "transformed" {
		t.Fatalf("published hook unavailable: %+v, %v", result, err)
	}
}

func TestRequestHookCannotSendPrivateThinkingMetadata(t *testing.T) {
	a := pluginFixture(t, `
wraith.hooks.add("before_request", function(ctx)
  ctx.request.think = "private.example"
  return {request=ctx.request}
end)`)
	client := a.providerClient(a.Config)
	client.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example"}}, nil)
	client.Profile.Kind = "plugin"
	_, err := client.Chat(t.Context(), ollama.Request{Model: "local"}, nil)
	if err == nil || !strings.Contains(err.Error(), "thinking metadata") || strings.Contains(err.Error(), "private.example") {
		t.Fatalf("lua hook bypassed thinking privacy: %v", err)
	}
}
