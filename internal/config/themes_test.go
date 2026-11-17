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

func TestHintColorsSupportExistingAndCustomPalettes(t *testing.T) {
	c := Default()
	custom := BuiltinThemes()["rose-pine"]
	custom.HintQuit, custom.HintRefresh, custom.HintCancel = "", "", ""
	c.Themes, c.Theme = map[string]Palette{"custom": custom}, "custom"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	p := c.Palette()
	if p.HintQuit == "" || p.HintRefresh == "" || p.HintCancel == "" {
		t.Fatal("existing palette lost hint defaults")
	}
	custom.HintQuit, custom.HintRefresh, custom.HintCancel = "#aa1122", "#2233bb", "#ddcc33"
	c.Themes["custom"] = custom
	path := File(t.TempDir())
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p := got.Palette(); p.HintQuit != custom.HintQuit || p.HintRefresh != custom.HintRefresh || p.HintCancel != custom.HintCancel {
		t.Fatal("custom hint colors did not survive save and load")
	}
	for _, field := range []*string{&custom.HintQuit, &custom.HintRefresh, &custom.HintCancel} {
		previous := *field
		*field = "invalid"
		c.Themes["custom"] = custom
		if c.Validate() == nil {
			t.Fatal("invalid hint color accepted")
		}
		*field = previous
	}
}
