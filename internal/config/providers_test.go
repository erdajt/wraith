package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProviderConfigurationRejectsMalformedEndpointsAndOverrides(t *testing.T) {
	for _, edit := range []func(*Config){
		func(c *Config) { c.Provider = "missing" },
		func(c *Config) { c.Providers["bad"] = Provider{Kind: "openai", BaseURL: "https://user:secret@host/v1"} },
		func(c *Config) { c.Providers["bad"] = Provider{Kind: "codex-cli"} },
		func(c *Config) {
			c.Providers["bad"] = Provider{Kind: "openai", BaseURL: "https://host/v1", Extra: map[string]any{"messages": "replace"}}
		},
	} {
		c := Default()
		edit(&c)
		if c.Validate() == nil {
			t.Fatal("invalid provider accepted")
		}
	}
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderThinkingValuesAndDefaultModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	text := "provider: codex\nthinking: xhigh\nproviders:\n  codex:\n    kind: codex-cli\n    command: [codex]\n    model: configured-model\n    thinking: [low, high, xhigh]\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || c.Model != "configured-model" || c.Thinking != "xhigh" {
		t.Fatalf("profile: %s %s %v", c.Model, c.Thinking, err)
	}
	c.Thinking = "unadvertised"
	if c.Validate() == nil {
		t.Fatal("unknown effort accepted")
	}
	if err := os.WriteFile(path, []byte(text+"model: explicit-model\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || c.Model != "explicit-model" {
		t.Fatal("explicit model overridden")
	}
}
