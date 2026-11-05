package ui

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestProviderSelectionPersistsBackendWithTheSession(t *testing.T) {
	a := fixture(t)
	a.Config.Providers["remote"] = config.Provider{Kind: "openai", BaseURL: "https://provider.invalid/v1", Models: []string{"remote-model"}, Model: "remote-model"}
	cmd := a.selectProvider("remote")
	if cmd == nil || a.Session.Provider != "remote" || a.Session.Model != "remote-model" || a.Client.Profile.Kind != "openai" {
		t.Fatal("provider switch did not prepare the selected model")
	}
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || saved.Provider != "remote" {
		t.Fatal("session lost provider")
	}
	loaded, err := config.LoadDirectory(a.Paths.Config)
	if err != nil || loaded.Provider != "remote" || loaded.Providers["remote"].Model != "remote-model" {
		t.Fatal("default provider not saved")
	}
}

func TestProviderSecretsStayOutOfSettingsAndInputHistory(t *testing.T) {
	a := fixture(t)
	p := a.Config.Providers["openai"]
	p.APIKey = "secret-profile-key"
	a.Config.Providers["openai"] = p
	a.settingsCategory("model")
	if strings.Contains(a.View(), p.APIKey) {
		t.Fatal("settings exposed api key")
	}
	a.rememberInput("/set providers.openai.api_key secret-profile-key")
	if len(a.inputHistory) != 0 {
		t.Fatal("credential command entered history")
	}
}
