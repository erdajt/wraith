package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaProviderSettingsPersistWithoutCopyingCredentials(t *testing.T) {
	for _, change := range []string{"model", "context", "provider"} {
		t.Run(change, func(t *testing.T) {
			a := pluginFixture(t, `
wraith.config.setup({providers={proof={kind="plugin",model="first",api_key="private-key",headers={authorization="private-header"}}}})
`)
			if change != "provider" {
				a.Config.Provider = "proof"
				a.Config.Model = "first"
				a.Session.Model = "first"
			}
			switch change {
			case "model":
				a.activateProvider("proof", "second")
			case "context":
				a.setContext("32768")
			case "provider":
				a.activateProvider("proof", "first")
			}
			saved, err := config.LoadDirectory(a.Paths.Config)
			if err != nil {
				t.Fatal(err)
			}
			profile := saved.Providers["proof"]
			if profile.Kind != "plugin" {
				t.Fatalf("profile was not saved: %+v; %s", profile, a.notice)
			}
			if change == "model" && (profile.Model != "second" || a.Session.Model != "second") {
				t.Fatalf("model selection was lost: %+v; %s", profile, a.notice)
			}
			if change == "context" && profile.ModelContexts["first"] != 32768 {
				t.Fatalf("model context was lost: %+v; %s", profile, a.notice)
			}
			data, err := os.ReadFile(config.File(a.Paths.Config))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "private-key") || strings.Contains(string(data), "private-header") {
				t.Fatal("lua credentials leaked into yaml")
			}
		})
	}
}

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
