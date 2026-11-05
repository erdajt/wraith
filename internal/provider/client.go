package provider

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
)

type Client struct {
	*ollama.Client
	Profile config.Provider
	Name    string
}

func New(c config.Config) *Client {
	p := c.Providers[c.Provider]
	base := p.BaseURL
	if p.Kind == "ollama" && base == "" {
		base = c.Host
	}
	return &Client{Client: ollama.New(base), Profile: p, Name: c.Provider}
}

func (c *Client) api() *API { return &API{Config: c.Profile, HTTP: c.HTTP} }

func (c *Client) Models(ctx context.Context) ([]ollama.Model, error) {
	if c.Profile.Kind == "ollama" {
		return c.Client.Models(ctx)
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

func (c *Client) Show(ctx context.Context, model string) (ollama.Info, error) {
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
		_, err := exec.LookPath(c.Profile.Command[0])
		return info, err
	}
	_, err := c.Models(ctx)
	return info, err
}

func (c *Client) EnsureServer(ctx context.Context, auto bool, path string) error {
	if c.Profile.Kind == "ollama" {
		return c.Client.EnsureServer(ctx, auto, path)
	}
	return nil
}

func (c *Client) Load(ctx context.Context, model, keepAlive string, options ollama.Options) error {
	if c.Profile.Kind == "ollama" {
		return c.Client.Load(ctx, model, keepAlive, options)
	}
	return nil
}

func (c *Client) Chat(ctx context.Context, request ollama.Request, emit func(ollama.Response)) (ollama.Response, error) {
	switch c.Profile.Kind {
	case "ollama":
		return c.Client.Chat(ctx, request, emit)
	case "openai":
		return c.api().OpenAI(ctx, request, emit)
	case "anthropic":
		return c.api().Anthropic(ctx, request, emit)
	case "codex-cli", "claude-cli":
		return CLI(ctx, c.Profile, request, emit)
	default:
		return ollama.Response{}, fmt.Errorf("unsupported provider kind: %s", c.Profile.Kind)
	}
}
