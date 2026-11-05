package config

import "testing"

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
