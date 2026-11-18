package ui

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestSettingsPreserveYamlUnderLuaOverrides(t *testing.T) {
	a := pluginFixture(t, `
wraith.config.set("theme", "nord")
wraith.config.set("providers.openai.api_key", "lua-only-key")
wraith.keymap.set("chat.normal", "x", function() end)
`)
	base := config.Default()
	base.Theme = "rose-pine"
	if err := config.Save(config.File(a.Paths.Config), base); err != nil {
		t.Fatal(err)
	}
	a.set("appearance.background", "inherit")
	saved, err := config.LoadDirectory(a.Paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Theme != "rose-pine" || saved.Appearance.Background != "inherit" || len(saved.Keymaps) != 0 || saved.Providers["openai"].APIKey == "lua-only-key" {
		t.Fatal("saving an unrelated setting copied lua overrides into yaml")
	}
	if a.Config.Theme != "nord" || len(a.Config.Keymaps) != 1 {
		t.Fatal("saving a setting discarded the effective lua configuration")
	}
	a.settingsCategory("appearance")
	for _, item := range a.popup.Items {
		if item.Value == "theme" && item.Detail == `"nord"` {
			t.Fatal("settings did not mark the lua override")
		}
	}
}
