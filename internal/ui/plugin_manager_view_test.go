package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/distribution"
	"github.com/erdajt/wraith/internal/plugin"
)

// managerViewApp attaches the distributed manager to an isolated ui fixture.
func managerViewApp(t *testing.T, source string) *App {
	t.Helper()
	selected, err := distribution.Prepare(t.Context(), "shroud", t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	return pluginFixture(t, source, plugin.PackageRoot{Directory: selected.Path, Namespaces: []string{"shroud", "wraith.manager"}})
}

const managerViewFixture = `local manager={}
local executed=0
local package={name="example",source={kind="git",url="https://example.com/plugin"},installed=true,loaded=false,state="installed",pin=true,commit=string.rep("a",40)}
manager.status=function() return {packages={package},operation={state="idle"}} end
manager.plan=function(action,names)
 assert(names[1]=="example")
 return {id="frozen",summary={action=action,packages={{name="dependency",source={kind="git",url="https://example.com/lib"},state="locked",commit=string.rep("a",40)}},pruned={"removed"}}}
end
manager.execute=function(id) assert(id=="frozen"); executed=executed+1 end
manager.cancel=function() wraith.notify("cancel requested") end
manager.load=function(name) assert(name=="example"); package.loaded=true; package.loaded_commit=string.rep("b",40) end
manager.unload=function(name) assert(name=="example"); package.loaded=false end
local view=require("wraith.manager.view").new(manager,{keys={refresh="R",confirm="a"}})
wraith.commands.create("packages",view.open)
wraith.commands.create("refresh-packages",view.refresh)
wraith.commands.create("close-packages",view.close)
wraith.commands.create("execution-count",function() wraith.notify(tostring(executed)) end)
`

func TestManagerViewUsesVimRowsAndExplicitFrozenPlanApproval(t *testing.T) {
	a := managerViewApp(t, managerViewFixture)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	reader := a.inputReader()
	if reader == nil || !strings.Contains(reader.Buffer.String(), "example · installed") {
		t.Fatalf("package view missing: %s", a.notice)
	}
	a.key(key("3"))
	a.key(key("j"))
	applyPluginEvents(t, a, a.key(key("i")))
	reader = a.inputReader()
	if reader == nil || !strings.Contains(reader.Buffer.String(), "remove lock entry: removed (files retained)") || !strings.Contains(reader.Buffer.String(), "dependency") {
		t.Fatalf("frozen plan missing dependencies or pruning: %s", a.notice)
	}
	if a.pluginDialog != nil || a.notice == "executed frozen plan" {
		t.Fatal("plan executed before review")
	}
	applyPluginEvents(t, a, a.key(key("a")))
	if a.pluginDialog == nil {
		t.Fatalf("confirmation missing: %s", a.notice)
	}
	applyPluginEvents(t, a, a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyEnter}))
	applyPluginEvents(t, a, a.runCommand("execution-count"))
	if a.notice != "1" {
		t.Fatalf("approved token not executed exactly once: %s", a.notice)
	}
}

func TestManagerViewRefreshKeepsFocusAndReopensClosedWindow(t *testing.T) {
	a := managerViewApp(t, managerViewFixture)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	a.key(key("3"))
	a.key(key("j"))
	applyPluginEvents(t, a, a.key(key("l")))
	if reader := a.inputReader(); reader == nil || !strings.Contains(reader.Buffer.String(), "example · loaded") {
		t.Fatalf("load status missing: %s", a.notice)
	}
	applyPluginEvents(t, a, a.runCommand("close-packages"))
	focus := a.focus
	applyPluginEvents(t, a, a.runCommand("refresh-packages"))
	if a.focus != focus {
		t.Fatal("background refresh stole focus")
	}
	applyPluginEvents(t, a, a.runCommand("packages"))
	if reader := a.inputReader(); reader == nil || !strings.Contains(reader.Buffer.String(), "example · loaded") {
		t.Fatalf("closed view did not reopen: %s", a.notice)
	}
}

func TestManagerViewCancelDoesNotExecutePlan(t *testing.T) {
	a := managerViewApp(t, managerViewFixture)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	a.key(key("3"))
	a.key(key("j"))
	applyPluginEvents(t, a, a.key(key("u")))
	applyPluginEvents(t, a, a.key(key("a")))
	if a.pluginDialog == nil {
		t.Fatalf("confirmation missing: %s", a.notice)
	}
	applyPluginEvents(t, a, a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyEscape}))
	applyPluginEvents(t, a, a.runCommand("execution-count"))
	if a.notice != "0" {
		t.Fatalf("canceled plan executed: %s", a.notice)
	}
}

func TestManagerDetailsRecoverAfterHostRejectsRegistration(t *testing.T) {
	a := managerViewApp(t, managerViewFixture+`wraith.keymap.set("chat.normal","z","noop")`)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	a.key(key("3"))
	a.key(key("j"))
	applyPluginEvents(t, a, a.key(tea.KeyMsg{Type: tea.KeyEnter}))
	before := a.plugins.Surfaces().Index()
	applyPluginEvents(t, a, a.runCommand("packages"))
	candidate := a.key(key("i"))().(pluginResultMsg)
	if candidate.Result.Manifest == nil {
		t.Fatal("plan did not publish confirmation binding")
	}
	previous := a.Config.Keymaps
	a.Config.Keymaps = append(a.Config.Keymaps, a.Config.Keymaps...)
	a.Update(candidate)
	a.Config.Keymaps = previous
	if !strings.Contains(a.notice, "keymaps changed") {
		t.Fatalf("host did not reject candidate: %s", a.notice)
	}
	applyPluginEvents(t, a, a.key(key("i")))
	after := a.plugins.Surfaces().Index()
	if len(after.Buffers) != len(before.Buffers) || len(after.Windows) != len(before.Windows) {
		t.Fatalf("rejected replacement leaked a pane: before=%+v after=%+v", before, after)
	}
	for index, item := range before.Buffers {
		if after.Buffers[index].ID != item.ID {
			t.Fatal("details buffer was replaced instead of reused")
		}
	}
	applyPluginEvents(t, a, a.runCommand("packages"))
	applyPluginEvents(t, a, a.key(tea.KeyMsg{Type: tea.KeyEnter}))
	applyPluginEvents(t, a, a.key(key("a")))
	if a.pluginDialog != nil {
		t.Fatal("inspect retained an old plan confirmation binding")
	}
}

func TestManagerViewRetainsWindowIdentityWhenCloseIsRejected(t *testing.T) {
	a := managerViewApp(t, managerViewFixture)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	id := a.plugins.Surfaces().Index().Windows[0].ID
	pending := a.runCommand("close-packages")().(pluginResultMsg)
	window, err := a.plugins.Surfaces().Window(id)
	if err != nil {
		t.Fatal(err)
	}
	window.Title = "newer title"
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.configure", ID: id, Owner: window.Owner, Lease: a.plugins.Surfaces().OwnerLease(window.Owner), Expected: window.Revision, Window: &window}); err != nil {
		t.Fatal(err)
	}
	a.Update(pending)
	if !strings.Contains(a.notice, "revision") {
		t.Fatalf("stale close was not rejected: %s", a.notice)
	}
	applyPluginEvents(t, a, a.runCommand("packages"))
	windows := a.plugins.Surfaces().Index().Windows
	if len(windows) != 1 || windows[0].ID != id {
		t.Fatalf("rejected close leaked a window: %+v", windows)
	}
}
