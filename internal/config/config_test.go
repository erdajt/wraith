package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTripAndPermissions(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Editor = []string{"nvim", "-u", "NONE"}
	c.Theme = "ember"
	if err = Save(p, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Editor[2] != "NONE" || got.Theme != "ember" {
		t.Fatalf("config lost fields: %+v", got)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
}

func TestRejectInvalidConfig(t *testing.T) {
	tests := map[string]func(*Config){
		"budget":    func(c *Config) { c.Predict = c.Context },
		"threshold": func(c *Config) { c.Compact.Threshold = 1.2 },
		"editor":    func(c *Config) { c.Editor = nil },
		"host":      func(c *Config) { c.Host = "file:///etc/passwd" },
		"keys":      func(c *Config) { c.Keys["send"] = c.Keys["quit"] },
		"thinking":  func(c *Config) { c.Thinking = "extreme" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c := Default()
			change(&c)
			if c.Validate() == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}

func TestUnknownConfigFieldsFail(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"verison":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("accepted misspelled field")
	}
}
