package provider

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/modeltool"
	"github.com/erdajt/wraith/internal/ollama"
	"github.com/erdajt/wraith/internal/variables"
)

type Client struct {
	Tools     *modeltool.Catalog
	Adapter   Adapter
	Transform func(context.Context, string, PromptData) (PromptData, error)
	Variables variables.Snapshot
	*ollama.Client
	Profile config.Provider
	Name    string
}

// New creates a client for the selected provider profile.
func New(c config.Config) *Client {
	p := c.Providers[c.Provider]
	base := p.BaseURL
	if p.Kind == "ollama" && base == "" {
		base = c.Host
	}
	return &Client{Client: ollama.New(base), Profile: p, Name: c.Provider}
}

// api builds an api transport with the client's profile and http client.
func (c *Client) api() *API { return &API{Config: c.Profile, HTTP: c.HTTP} }

// Models discovers models through the selected provider after checking private metadata.
func (c *Client) Models(ctx context.Context) ([]ollama.Model, error) {
	private, _, err := c.privateRequest(ollama.Request{})
	if err != nil {
		return nil, err
	}
	c = private
	if c.Profile.Kind == "plugin" {
		if c.Adapter == nil {
			return nil, fmt.Errorf("plugin provider %s is unavailable", c.Name)
		}
		return c.Adapter.Models(ctx)
	}

	if c.Profile.Kind == "ollama" {
		return c.Client.Models(ctx)
	}

	if c.Profile.Kind == "codex-cli" && (len(c.Profile.Models) == 0 || len(c.Profile.Models) == 1 && c.Profile.Models[0] == "default") {
		catalog, err := codexCatalog(ctx, c.Profile)
		if err != nil {
			return nil, err
		}

		models := make([]ollama.Model, 0, len(catalog))
		for _, model := range catalog {
			models = append(models, ollama.Model{Name: model.Name})
		}
		return models, nil
	}

	if strings.HasSuffix(c.Profile.Kind, "-cli") {
		names := c.Profile.Models
		if len(names) == 0 {
			names = []string{"default"}
		}

		var models []ollama.Model
		for _, name := range names {
			models = append(models, ollama.Model{Name: name})
		}
		return models, nil
	}

	return c.api().Models(ctx)
}

// Show loads model metadata or checks provider readiness and resolves thinking options.
func (c *Client) Show(ctx context.Context, model string) (ollama.Info, error) {
	private, _, privacyErr := c.privateRequest(ollama.Request{Model: model})
	if privacyErr != nil {
		return ollama.Info{}, privacyErr
	}
	c = private
	if c.Profile.Kind == "plugin" {
		if c.Adapter == nil {
			return ollama.Info{}, fmt.Errorf("plugin provider %s is unavailable", c.Name)
		}
		return c.Adapter.Show(ctx, model)
	}

	if c.Profile.Kind == "ollama" {
		return c.Client.Show(ctx, model)
	}

	info := ollama.Info{Capabilities: []string{"completion"}}
	for _, value := range c.Profile.Thinking {
		switch value {
		case "on":
			info.Thinking.Values = append(info.Thinking.Values, true)
		case "off":
			info.Thinking.Values = append(info.Thinking.Values, false)
		default:
			info.Thinking.Values = append(info.Thinking.Values, value)
		}
	}

	if strings.HasSuffix(c.Profile.Kind, "-cli") {
		if len(c.Profile.Command) == 0 {
			return info, fmt.Errorf("cli command is empty")
		}

		_, err := exec.LookPath(c.Profile.Command[0])
		if err == nil && c.Profile.Kind == "codex-cli" {
			catalog, _ := codexCatalog(ctx, c.Profile)
			info = codexInfo(catalog, model, info)
		}
		return info, err
	}

	_, err := c.Models(ctx)
	return info, err
}

// EnsureServer checks or starts the server for ollama profiles; other providers need no server.
func (c *Client) EnsureServer(ctx context.Context, auto bool, path string) error {
	if c.Profile.Kind == "ollama" {
		private, _, err := c.privateRequest(ollama.Request{})
		if err != nil {
			return err
		}
		return private.Client.EnsureServer(ctx, auto, path)
	}
	return nil
}

// Load warms an ollama model; other providers need no preload.
func (c *Client) Load(ctx context.Context, model, keepAlive string, options ollama.Options) error {
	if c.Profile.Kind == "ollama" {
		private, _, err := c.privateRequest(ollama.Request{Model: model})
		if err != nil {
			return err
		}
		return private.Client.Load(ctx, model, keepAlive, options)
	}
	return nil
}

// Chat redacts the request, checks its budget and streams it through the selected provider.
func (c *Client) Chat(ctx context.Context, request ollama.Request, emit func(ollama.Response)) (ollama.Response, error) {
	request, err := c.transformRequest(ctx, request)
	if err != nil {
		return ollama.Response{}, err
	}
	private, sanitized, err := c.privateRequest(request)
	if err != nil {
		return ollama.Response{}, err
	}
	c, request = private, sanitized

	input, err := requestTokens(request)
	if err != nil {
		return ollama.Response{}, err
	}
	if err := CheckContext(input, request.Options.Predict, request.Options.Context); err != nil {
		return ollama.Response{}, err
	}

	var result ollama.Response
	switch c.Profile.Kind {
	case "plugin":
		if c.Adapter == nil {
			return ollama.Response{}, fmt.Errorf("plugin provider %s is unavailable", c.Name)
		}
		result, err = c.Adapter.Chat(ctx, request, emit)
	case "ollama":
		result, err = c.Client.Chat(ctx, request, emit)
	case "openai":
		result, err = c.api().OpenAI(ctx, request, emit)
	case "anthropic":
		result, err = c.api().Anthropic(ctx, request, emit)
	case "codex-cli", "claude-cli":
		return CLI(ctx, c.Profile, request, emit)
	default:
		return ollama.Response{}, fmt.Errorf("unsupported provider kind: %s", c.Profile.Kind)
	}

	calls := result.Calls
	result.Calls = nil
	if err != nil || len(calls) == 0 {
		return result, err
	}
	if !result.Done || result.Reason == "length" || len(request.Tools) == 0 {
		return result, &ollama.ActionError{Problem: "native action was incomplete or unavailable"}
	}
	content, err := contextual.NormalizeAction(request.ExecutionMode, result.Message.Content, calls, c.extensionCatalog(request))
	if err != nil {
		return result, &ollama.ActionError{Problem: err.Error()}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	result.Message.Content = content
	return result, nil
}
