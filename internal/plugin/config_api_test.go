package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestLuaConfigUsesRealExpressionsAndDetachedSettings(t *testing.T) {
	dir := t.TempDir()
	source := `
local c = wraith.config.get()
c.context = 12
wraith.config.setup({context = 32 * 1024, appearance = {background = "inherit"}})
assert(wraith.config.get("context") == 32768)
assert(wraith.config.get("appearance.mouse"))
local colors = wraith.config.get("themes")
colors.custom = {bg="#101010", fg="#eeeeee", muted="#888888", accent="#00ff00", second="#ff00ff", border="#444444", warn="#ff8800"}
wraith.config.setup({themes = colors, theme = "custom"})
`
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	base := config.Default()
	r, err := Open(t.Context(), dir, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	got := r.Manifest().Config
	if got.Context != 32768 || got.Theme != "custom" || got.Appearance.Background != "inherit" || !got.Appearance.Mouse || base.Context != 8192 {
		t.Fatalf("unexpected effective configuration: %+v", got)
	}
	got.Themes["custom"] = config.Palette{}
	if r.Manifest().Config.Themes["custom"].BG != "#101010" {
		t.Fatal("manifest shared mutable configuration")
	}
}
