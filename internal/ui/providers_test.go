package ui

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/ollama"
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

func TestModelCatalogIgnoresStaleAndDismissedRequests(t *testing.T) {
	a := fixture(t)
	a.modelCommand("models", "")
	old := modelsMsg{Serial: a.catalogSerial, Models: []ollama.Model{{Name: "old-model"}}}
	a.Config.Providers["remote"] = config.Provider{Kind: "openai", BaseURL: "https://provider.invalid", Model: "new-model"}
	a.activateProvider("remote", "new-model")
	a.popup = &menu{Kind: "settings", Title: "settings"}
	a.Update(old)
	if a.popup.Kind != "settings" || a.Session.Model != "new-model" {
		t.Fatal("old catalog replaced provider state")
	}
	a.modelCommand("models", "")
	dismissed := modelsMsg{Serial: a.catalogSerial, Models: old.Models}
	a.popup = nil
	a.Update(dismissed)
	if a.popup != nil {
		t.Fatal("dismissed catalog reopened")
	}
	a.modelCommand("models", "")
	a.Update(modelsMsg{Serial: a.catalogSerial, Models: []ollama.Model{{Name: "valid-model"}}})
	if a.popup == nil || a.popup.Kind != "models" || a.popup.Items[0].Value != "valid-model" {
		t.Fatal("current catalog was lost")
	}
}

func TestProviderCatalogCannotReplaceNewerPicker(t *testing.T) {
	a := fixture(t)
	a.selectProvider("openai")
	old := providerModelsMsg{Serial: a.catalogSerial, Name: "openai", Models: []ollama.Model{{Name: "old"}}}
	a.selectProvider("anthropic")
	a.providerModels(old)
	if a.popup.Kind != "provider-loading" || !strings.Contains(a.popup.Title, "anthropic") {
		t.Fatal("stale provider catalog replaced pending selection")
	}
}
