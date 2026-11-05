package config

import "testing"

func TestCustomThemesRoundTripAndValidate(t *testing.T) {
	c := Default()
	for name := range BuiltinThemes() {
		c.Theme = name
		if err := c.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	custom := BuiltinThemes()["rose-pine"]
	custom.Accent = "#123456"
	c.Themes = map[string]Palette{"mine": custom}
	c.Theme = "mine"
	path := File(t.TempDir())
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Palette().Accent != "#123456" {
		t.Fatalf("custom theme lost: %v", err)
	}
	custom.Warn = "invalid"
	c.Themes["mine"] = custom
	if c.Validate() == nil {
		t.Fatal("invalid theme accepted")
	}
}
