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
		"budget":    func(c *Config) { c.Predict = 0 },
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

func TestRejectMalformedTrailingData(t *testing.T) {
	for _, suffix := range []string{"false", "{", "garbage"} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(`{"version":1}`+suffix), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted trailing %q", suffix)
		}
	}
}

func TestContextHasNoApplicationCeiling(t *testing.T) {
	for _, size := range []int{1, 8192, 32768, 1000000} {
		c := Default()
		c.Context = size
		if err := c.Validate(); err != nil {
			t.Fatalf("context %d: %v", size, err)
		}
	}
	for _, size := range []int{0, -1} {
		c := Default()
		c.Context = size
		if c.Validate() == nil {
			t.Fatalf("accepted context %d", size)
		}
	}
}
