package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestManagerOperationLogUsesBoundedSearchableBuffer(t *testing.T) {
	a := managerViewApp(t, managerViewFixture+`
manager.status=function() return {packages={package},operation={
 id="transaction",state="fetching",name="example",message="fetching exact revision",
 log=string.rep("x",9000).."\nprogress\n"..string.char(27).."end",log_truncated=true,
}} end
`)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	reader := a.inputReader()
	if reader == nil || !strings.Contains(reader.Buffer.String(), "active package: example") || !strings.Contains(reader.Buffer.String(), "fetching exact revision") {
		t.Fatalf("operation progress missing: %s", a.notice)
	}
	applyPluginEvents(t, a, a.key(key("o")))
	reader = a.inputReader()
	if reader == nil {
		t.Fatal("operation log did not open")
	}
	text := reader.Buffer.String()
	for _, want := range []string{"operation: transaction", "state: fetching", "progress", "[log excerpt truncated]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("log missing %q", want)
		}
	}
	if len(text) > 8500 || strings.ContainsRune(text, '\x1b') {
		t.Fatal("log retained excessive bytes or terminal controls")
	}
	a.key(key("/"))
	for _, char := range "progress" {
		a.key(key(string(char)))
	}
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if reader.Buffer.Pos != strings.Index(text, "progress") {
		t.Fatal("log search did not move the vim cursor")
	}
	applyPluginEvents(t, a, a.key(key("q")))
	if reader := a.inputReader(); reader == nil || !strings.HasPrefix(reader.Buffer.String(), "packages") {
		t.Fatal("closing the log did not return to package status")
	}
}

// TestManagerCleanupWarningPreservesInstalledPresentation keeps recovery visible without misreporting publication.
func TestManagerCleanupWarningPreservesInstalledPresentation(t *testing.T) {
	a := managerViewApp(t, managerViewFixture+`
manager.status=function() return {packages={package},operation={
 id="transaction",state="installed",phase="cleaning",generation="published",
 cleanup_warning="temporary checkout remains; inspect recovery paths",
}} end
`)
	a.width, a.height = 120, 40
	applyPluginEvents(t, a, a.runCommand("packages"))
	text := a.inputReader().Buffer.String()
	if !strings.HasPrefix(text, "packages · installed") || !strings.Contains(text, "cleanup: temporary checkout remains") {
		t.Fatalf("publication or cleanup state missing: %s", text)
	}

	applyPluginEvents(t, a, a.key(key("o")))
	text = a.inputReader().Buffer.String()
	if !strings.Contains(text, "state: installed") || !strings.Contains(text, "cleanup: temporary checkout remains") {
		t.Fatalf("operation details lost cleanup warning: %s", text)
	}
}
