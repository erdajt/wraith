package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestManagerAdoptsLateLocalInstallation loads a directory created after configuration without git storage.
func TestManagerAdoptsLateLocalInstallation(t *testing.T) {
	directory := t.TempDir()
	root, storage := filepath.Join(directory, "late"), filepath.Join(directory, "packages")
	lockfile := filepath.Join(directory, "lock.json")
	r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
manager.setup({{name="late",dir=%q,lazy=true,opts={}}},{storage={root=%q,lockfile=%q}})
assert(not manager.status().packages[1].installed)
wraith.commands.create("adopt", function()
 manager.load("late")
 local row = manager.status().packages[1]
 assert(row.loaded and row.installed and not row.restart_required)
 assert(row.main_source_path == %q and row.main_source_kind == "file")
 assert(require("late").value == 42)
end)
`, root, storage, lockfile, filepath.Join(root, "lua", "late.lua")))
	if err := os.MkdirAll(filepath.Join(root, "lua"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lua", "late.lua"), []byte(`return {value=42,setup=function()
 wraith.commands.create("late-owned", function() wraith.notify("late source") end)
end}`), 0o600); err != nil {
		t.Fatal(err)
	}

	managerCommand(t, r, "adopt", "")
	result := managerCommand(t, r, "late-owned", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "late source" {
		t.Fatalf("late source did not own its command: %+v", result)
	}
	for _, absent := range []string{storage, lockfile} {
		if _, err := os.Stat(absent); !os.IsNotExist(err) {
			t.Fatalf("local adoption changed managed storage: %s: %v", absent, err)
		}
	}
}

// TestManagerAdoptsCommittedInstallAfterRejectedStoreCompletion retains disk publication without accepting registrations.
func TestManagerAdoptsCommittedInstallAfterRejectedStoreCompletion(t *testing.T) {
	source, _ := managerGitFixture(t)
	if err := os.Mkdir(filepath.Join(source, "lua"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "lua", "adoption.lua"), []byte(`source_runs=(source_runs or 0)+1
return {setup=function()
 wraith.commands.create("adoption-owned", function() wraith.notify(source_runs) end)
end}`), 0o600); err != nil {
		t.Fatal(err)
	}
	managerGit(t, source, "add", "lua")
	managerGit(t, source, "commit", "--quiet", "-m", "adoption fixture")
	commit := managerGit(t, source, "rev-parse", "HEAD")
	directory := t.TempDir()
	storage, lockfile := filepath.Join(directory, "packages"), filepath.Join(directory, "lock.json")
	r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
local storage = {root=%q,lockfile=%q}
manager.setup({{name="adoption",url=%q,lazy=true,opts={}}},{storage=storage})
assert(not manager.status().packages[1].installed)
wraith.commands.create("adopt", function()
 manager.load("adoption")
 local row = manager.status().packages[1]
 assert(row.loaded and row.loaded_commit == %q and not row.restart_required)
 assert(require("adoption").setup)
end)
local store = require("wraith.manager.store").new({{
 name="adoption",source={kind="git",url=%q},selector={kind="head"},pin=false,
}}, storage)
local plan = store.plan("install", {"adoption"})
store.execute(plan, nil, function(result)
 assert(result.state == "installed", result.error)
 wraith.commands.create("installation-completed", function() end)
 wraith.notify("committed")
end)
`, storage, lockfile, source, commit, source))
	if err := r.Activate(r.Manifest().Resources); err != nil {
		t.Fatal(err)
	}

	for {
		event := receiveResource(t, r)
		if !r.ResourceValid(event) {
			continue
		}
		pending, err := r.SubmitResource(event, nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		completed := result.Manifest != nil && slices.ContainsFunc(result.Manifest.Commands, func(command Command) bool {
			return command.Name == "installation-completed"
		})
		if result.Manifest != nil {
			if err := r.Acknowledge(result.Revision, !completed); err != nil {
				t.Fatal(err)
			}
		}
		if completed {
			break
		}
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			default:
				t.Fatalf("unexpected installation effect: %+v", effect)
			}
		}
	}

	lock, err := os.ReadFile(lockfile)
	if err != nil || !strings.Contains(string(lock), commit) {
		t.Fatalf("rejected completion lost committed installation: %s: %v", lock, err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	var callback string
	for _, command := range r.Manifest().Commands {
		if command.Name == "installation-completed" {
			t.Fatal("rejected completion retained its marker")
		}
		if command.Name == "adopt" {
			callback = command.Callback
		}
	}
	if callback == "" {
		t.Fatal("adoption command missing")
	}
	pending, err := r.Submit(callback, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Manifest == nil {
		t.Fatalf("installed source was not adopted: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, false); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(r.Manifest().Commands, func(command Command) bool { return command.Name == "adoption-owned" }) {
		t.Fatal("rejected activation retained source registration")
	}

	managerCommand(t, r, "adopt", "")
	result = managerCommand(t, r, "adoption-owned", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "2" {
		t.Fatalf("retry did not rerun the installed source once: %+v", result)
	}
	after, err := os.ReadFile(lockfile)
	if err != nil || string(after) != string(lock) {
		t.Fatalf("adoption refetched or rewrote the committed lock: %v", err)
	}
}
