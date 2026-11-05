package ui

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/session"
)

func TestPaneSettingsPersistAndResizeGeometry(t *testing.T) {
	a := fixture(t)
	beforeW, beforeH := a.paneSize(0)
	a.settingMenu("terminal.overlay_width")
	if len(a.popup.Items) == 0 {
		t.Fatal("missing width choices")
	}
	a.set("terminal.overlay_width", "0.9")
	a.set("terminal.overlay_height", "0.9")
	afterW, afterH := a.paneSize(0)
	if afterW <= beforeW || afterH <= beforeH {
		t.Fatal("pane geometry did not grow")
	}
	c, err := config.Load(a.Paths.Config + "/config.yaml")
	if err != nil || c.Terminal.OverlayWidth != 0.9 || c.Terminal.OverlayHeight != 0.9 {
		t.Fatalf("not persisted: %+v %v", c, err)
	}
	a.set("terminal.split_width", "0.99")
	if a.Config.Terminal.SplitWidth != 0.4 {
		t.Fatal("invalid width accepted")
	}
}

func TestClearKeepsPreviousSessionAvailable(t *testing.T) {
	a := fixture(t)
	previous := a.Session.ID
	a.Session.Messages = []session.Message{{Role: "user", Content: "retain this"}}
	a.Instructions = "persistent preference"
	a.runCommand("clear")
	if a.Session.ID == previous || len(a.Session.Messages) != 0 || a.Instructions != "persistent preference" {
		t.Fatal("clear did not create a clean chat")
	}
	saved, err := a.Store.Load(previous)
	if err != nil || saved.Messages[0].Content != "retain this" {
		t.Fatalf("previous chat lost: %v", err)
	}
}
