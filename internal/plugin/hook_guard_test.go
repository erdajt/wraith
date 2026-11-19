package plugin

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestPureHookGuardRejectsCapturedHostFunctions(t *testing.T) {
	e, err := load(t.Context(), t.TempDir(), config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.L.Close)
	e.L.SetContext(t.Context())
	if err := e.L.DoString(`captured=wraith.resource.cancel`); err != nil {
		t.Fatal(err)
	}
	e.pureHook = true
	if err := e.L.DoString(`assert(not pcall(captured,"missing")); assert(wraith.json.decode('{"ok":true}').ok)`); err != nil {
		t.Fatal(err)
	}
	if e.hookViolation == nil || len(e.effects) != 0 {
		t.Fatal("captured host operation escaped pure hook guard")
	}
	e.hookViolation = nil
	if err := e.L.DoString(`assert(not pcall(print,"host notice"))`); err != nil {
		t.Fatal(err)
	}
	if e.hookViolation == nil || len(e.effects) != 0 {
		t.Fatal("global print emitted an operation during a hook")
	}
	e.pureHook = false
	if err := e.L.DoString(`wraith.notify("normal callback")`); err != nil {
		t.Fatal(err)
	}
	if len(e.effects) != 1 {
		t.Fatal("hook guard disabled ordinary callbacks")
	}
}
