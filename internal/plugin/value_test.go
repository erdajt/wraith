package plugin

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

func TestLuaValuesRejectCyclesAndMixedTables(t *testing.T) {
	L := lua.NewState()
	t.Cleanup(L.Close)
	for _, source := range []string{`v = {}; v.self = v`, `v = {[1] = "one", label = "mixed"}`, `v = {[2] = "missing"}`} {
		if err := L.DoString(source); err != nil {
			t.Fatal(err)
		}
		if _, err := decodeValue(L.GetGlobal("v")); err == nil {
			t.Fatalf("accepted invalid value from %s", source)
		}
	}
}

func TestConfigChangesValidateWithoutMutatingOriginal(t *testing.T) {
	original := config.Default()
	next, err := changeConfig(original, "appearance.background", "inherit")
	if err != nil || next.Appearance.Background != "inherit" || original.Appearance.Background != "theme" {
		t.Fatalf("nested update: %+v, %v", next.Appearance, err)
	}
	for _, tc := range []struct {
		path  string
		value any
	}{{"context", -1}, {"appearance.typo", true}, {"appearance", map[string]any{"unknown": true}}, {"editor", "nvim"}} {
		if _, err := changeConfig(original, tc.path, tc.value); err == nil {
			t.Errorf("accepted %s = %v", tc.path, tc.value)
		}
	}
	if next, err := changeConfig(original, "keymaps", map[string]any{}); err != nil || len(next.Keymaps) != 0 {
		t.Fatalf("empty array: %v", err)
	}
}
