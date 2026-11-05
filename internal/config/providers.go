package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

type Provider struct {
	Kind         string            `json:"kind" yaml:"kind"`
	BaseURL      string            `json:"base_url" yaml:"base_url"`
	APIKey       string            `json:"api_key" yaml:"api_key"`
	KeyEnv       string            `json:"key_env" yaml:"key_env"`
	Headers      map[string]string `json:"headers" yaml:"headers"`
	Model        string            `json:"model" yaml:"model"`
	Models       []string          `json:"models" yaml:"models"`
	Command      []string          `json:"command" yaml:"command"`
	SystemPrompt string            `json:"system_prompt" yaml:"system_prompt"`
	Thinking     []string          `json:"thinking" yaml:"thinking"`
	ThinkingMode string            `json:"thinking_mode" yaml:"thinking_mode"`
	TokenField   string            `json:"token_field" yaml:"token_field"`
	Extra        map[string]any    `json:"extra" yaml:"extra"`
}

func DefaultProviders() map[string]Provider {
	return map[string]Provider{
		"ollama":    {Kind: "ollama"},
		"openai":    {Kind: "openai", BaseURL: "https://api.openai.com/v1", KeyEnv: "OPENAI_API_KEY", TokenField: "max_completion_tokens"},
		"anthropic": {Kind: "anthropic", BaseURL: "https://api.anthropic.com/v1", KeyEnv: "ANTHROPIC_API_KEY"},
		"deepseek":  {Kind: "openai", BaseURL: "https://api.deepseek.com", KeyEnv: "DEEPSEEK_API_KEY", TokenField: "max_tokens"},
		"codex":     {Kind: "codex-cli", Command: []string{"codex"}, Models: []string{"default"}, Model: "default"},
		"claude":    {Kind: "claude-cli", Command: []string{"claude"}, Models: []string{"default"}, Model: "default"},
	}
}

func (c Config) ValidateProviders() error {
	if _, ok := c.Providers[c.Provider]; !ok {
		return fmt.Errorf("unknown provider: %s", c.Provider)
	}
	for name, p := range c.Providers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("provider names cannot be empty")
		}
		if !slices.Contains([]string{"ollama", "openai", "anthropic", "codex-cli", "claude-cli"}, p.Kind) {
			return fmt.Errorf("provider %s has an unsupported kind", name)
		}
		if strings.HasSuffix(p.Kind, "-cli") {
			if len(p.Command) == 0 || strings.TrimSpace(p.Command[0]) == "" {
				return fmt.Errorf("provider %s needs a command", name)
			}
		} else if p.BaseURL != "" || p.Kind != "ollama" {
			u, err := url.Parse(p.BaseURL)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return fmt.Errorf("provider %s needs an http base_url without credentials or query parameters", name)
			}
		}
		if !slices.Contains([]string{"", "reasoning_effort", "thinking", "adaptive"}, p.ThinkingMode) {
			return fmt.Errorf("provider %s has an unsupported thinking_mode", name)
		}
		if !slices.Contains([]string{"", "max_tokens", "max_completion_tokens"}, p.TokenField) {
			return fmt.Errorf("provider %s has an unsupported token_field", name)
		}
		for key := range p.Extra {
			if slices.Contains([]string{"messages", "system", "model", "stream", "tools", "tool_choice"}, key) {
				return fmt.Errorf("provider %s extra cannot replace %s", name, key)
			}
		}
	}
	return nil
}
