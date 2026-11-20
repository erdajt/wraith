package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestManagerRequireActivatesInstalledSources exercises the public manager through native require.
func TestManagerRequireActivatesInstalledSources(t *testing.T) {
	for _, mode := range []string{"default", "custom", "library", "init", "coroutine"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "lua"), 0o700); err != nil {
				t.Fatal(err)
			}
			source := `source_calls = (source_calls or 0) + 1
assert(coroutine.running() == expected_thread)
wraith.commands.create("fixture.source", function() end)
wraith.events.on("ready", function() wraith.notify("source ready") end)
return { value = 42, setup = function(opts)
 assert(opts.token == "kept" and coroutine.running() == expected_thread)
 setup_calls = (setup_calls or 0) + 1
 wraith.events.on("ready", function() wraith.notify("setup ready") end)
end }
`
			if err := os.WriteFile(filepath.Join(root, "lua", "require_fixture.lua"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			configuration := `opts = { token = "kept" }`
			setups := 1
			switch mode {
			case "custom":
				configuration = `config = function()
 assert(require("require_fixture").value == 42)
 setup_calls = (setup_calls or 0) + 1
 wraith.events.on("ready", function() wraith.notify("setup ready") end)
end`
			case "library":
				configuration = ""
				setups = 0
			case "init":
				configuration += `, init = function()
 assert(source_calls == nil)
 assert(require("require_fixture").value == 42)
 assert(source_calls == 1 and setup_calls == 1)
end`
			}
			r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
manager.setup({ { name = "require-fixture", dir = %q, lazy = true, %s } })
wraith.commands.create("import", function()
 local function load()
  expected_thread = coroutine.running()
  local value = require("require_fixture")
  assert(value.value == 42 and value == require("require_fixture"))
  assert(source_calls == 1 and (setup_calls or 0) == %d)
  assert(manager.status().packages[1].loaded)
 end
 if %t then
  local ok, err = coroutine.resume(coroutine.create(load))
  assert(ok, err)
 else
  load()
 end
end)
`, root, configuration, setups, mode == "coroutine"))

			pending, err := r.Submit("", "ready", nil, r.Manifest().Config)
			if err != nil {
				t.Fatal(err)
			}
			notices := drainManagerEvents(t, r, receiveResult(t, pending))
			for attempt := 0; attempt < 2; attempt++ {
				var callback string
				for _, command := range r.Manifest().Commands {
					if command.Name == "import" {
						callback = command.Callback
					}
				}
				if callback == "" {
					t.Fatal("import command missing")
				}
				pending, err = r.Submit(callback, "", nil, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				notices = append(notices, drainManagerEvents(t, r, receiveResult(t, pending))...)
			}

			want := []string{"setup ready", "source ready"}
			if mode == "library" {
				want = []string{"source ready"}
			}
			slices.Sort(notices)
			if !slices.Equal(notices, want) {
				t.Fatalf("require readiness: got %v, want %v", notices, want)
			}
		})
	}
}
