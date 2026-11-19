package ui

import (
	"context"
	"errors"
	"slices"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/provider"
	"github.com/erdajt/wraith/internal/session"
)

// providerClient binds immutable workspace identity to the selected extension runtime.
func (a *App) providerClient(c config.Config, runtimes ...*plugin.Runtime) *provider.Client {
	client := provider.New(c)
	client.Variables = a.Variables
	runtime := a.plugins
	if len(runtimes) > 0 {
		runtime = runtimes[0]
	}
	if runtime == nil {
		return client
	}
	if !slices.ContainsFunc(runtime.Manifest().Hooks, func(hook plugin.HookDefinition) bool { return hook.Kind == "before_request" }) {
		return client
	}
	sessionID, directory := a.Session.ID, a.Directory
	client.Transform = func(ctx context.Context, purpose string, data provider.PromptData) (provider.PromptData, error) {
		request := plugin.RequestData{Model: data.Model, Think: data.Think, Options: data.Options}
		for _, message := range data.Messages {
			request.Messages = append(request.Messages, plugin.HookMessage{Role: message.Role, Content: message.Content, Reference: message.Reference})
		}
		result, err := runtime.Transform(ctx, plugin.HookRequest{
			Kind: "before_request", Phase: "prepare", Purpose: purpose,
			ID: session.ID(), SessionID: sessionID, Directory: directory, Request: &request,
		})
		if err != nil {
			return data, err
		}
		if result.Decision == "deny" {
			return data, errors.New("denied: " + result.Reason)
		}
		if result.Request == nil {
			return data, errors.New("request hook returned no request")
		}
		data = provider.PromptData{Model: result.Request.Model, Think: result.Request.Think, Options: result.Request.Options}
		for _, message := range result.Request.Messages {
			data.Messages = append(data.Messages, provider.PromptMessage{Role: message.Role, Content: message.Content, Reference: message.Reference})
		}
		return data, nil
	}
	return client
}
