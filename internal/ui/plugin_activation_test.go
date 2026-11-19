package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLuaAbandonedHostActionsDiscardReservedFilesystemWork(t *testing.T) {
	for _, failure := range []string{"blocked prompt", "failed editor"} {
		t.Run(failure, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "unaccepted")
			action := `wraith.prompt.set("blocked")`
			if failure == "failed editor" {
				action = `wraith.action("editor.open")`
			}
			a := pluginFixture(t, fmt.Sprintf(`local reserved
wraith.commands.create("start", function()
 %s
 reserved = wraith.fs.write_async(%q, "unaccepted", function() error("must not run") end)
end)
wraith.commands.create("check", function()
 assert(not pcall(wraith.resource.status, reserved))
 wraith.notify("released")
end)`, action, path))
			result := a.runCommand("start")().(pluginResultMsg)
			if failure == "blocked prompt" {
				a.popup = &menu{Kind: "approval"}
				a.applyPluginResult(result)
				a.popup = nil
			} else {
				a.applyPluginResult(result)
				if !a.editing || a.editorFile == nil {
					t.Fatal("editor did not hold the remaining resource effect")
				}
				a.Update(editedMsg{Kind: "prompt", File: a.editorFile, Err: errors.New("editor failed")})
			}
			a.Update(a.runCommand("check")())
			if a.notice != "released" {
				t.Fatalf("abandoned resource remained reserved: %s", a.notice)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("unaccepted filesystem operation ran: %v", err)
			}
		})
	}
}

func TestLuaRejectedStartupCannotActivateFilesystemWork(t *testing.T) {
	a := fixture(t)
	path := filepath.Join(t.TempDir(), "unaccepted")
	source := fmt.Sprintf(`wraith.fs.write_async(%q, "unaccepted", function() end)
wraith.config.set("keys.editor", "<C-s>")`, path)
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	runtime, err := LoadPlugin(t.Context(), a.Paths.Config, a.Config, a.Paths)
	if runtime != nil {
		runtime.Close()
		t.Fatal("invalid host mapping was installed")
	}
	if err == nil || !strings.Contains(err.Error(), "conflicting default key") {
		t.Fatalf("candidate did not fail host validation: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("rejected startup performed external work: %v", err)
	}
}

func TestLuaServiceCanReleaseItselfAfterItsFinalUpdate(t *testing.T) {
	a := pluginFixture(t, `wraith.timer.start(1, function(ctx)
 wraith.prompt.set("finished")
 wraith.resource.release(ctx.resource.id)
 wraith.notify("complete")
end)`)
	result := resourceResult(t, a)
	a.applyPluginResult(result.Result)
	if a.Prompt.String() != "finished" || a.notice != "complete" || a.plugins.ResourceValid(*result.Result.Resource) {
		t.Fatal("self release lost earlier effects or retained the completed handle")
	}
}
