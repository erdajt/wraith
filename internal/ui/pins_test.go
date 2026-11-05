package ui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/editor"
)

func TestFilteredPinRemovalUsesOriginalIndex(t *testing.T) {
	a := fixture(t)
	a.Session.Pins = []string{"first", "needle", "last"}
	a.memoryCommand("pins", "")
	a.key(key("/"))
	a.key(key("needle"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	a.key(key("d"))
	if len(a.Session.Pins) != 2 || a.Session.Pins[1] != "last" {
		t.Fatal("wrong pin removed")
	}
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || len(saved.Pins) != 2 {
		t.Fatal("pin removal not saved")
	}
}

func TestSavedPinEditPreservesOtherPins(t *testing.T) {
	a := fixture(t)
	a.Session.Pins = []string{"first", "old"}
	f, err := editor.Prepare(a.Paths.Cache, ".md", "old")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.Path, []byte("exact revised fact\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "pin", Index: 1, File: f})
	if a.Session.Pins[0] != "first" || a.Session.Pins[1] != "exact revised fact" || a.popup.Kind != "pins" {
		t.Fatal("pin edit failed")
	}
}
