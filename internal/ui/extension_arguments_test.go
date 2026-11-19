package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/modeltool"
	"github.com/erdajt/wraith/internal/variables"
)

func TestExtensionArgumentsPreserveJSONStructureAcrossPrivateExpansion(t *testing.T) {
	schema, err := modeltool.Compile([]byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "quote\"\nline"}}, nil)
	data, err := resolveToolArguments([]byte(`{"items":["wraith::TARGET",null,{},[]],"number":9007199254740991}`), snapshot, schema)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Items  []json.RawMessage `json:"items"`
		Number json.Number       `json:"number"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	var value string
	if err = json.Unmarshal(result.Items[0], &value); err != nil || value != "quote\"\nline" || string(result.Items[1]) != "null" || string(result.Items[2]) != "{}" || string(result.Items[3]) != "[]" || result.Number.String() != "9007199254740991" {
		t.Fatalf("argument structure changed: %s, %v", data, err)
	}
	for _, invalid := range []string{`{"wraith::TARGET":"value"}`, `{"value":"wraith::UNKNOWN"}`, `{"value":1,"value":2}`} {
		if _, err = resolveToolArguments([]byte(invalid), snapshot, schema); err == nil {
			t.Fatalf("invalid argument accepted: %s", invalid)
		}
	}
}

func TestExtensionApprovalKeepsItsPrivateSnapshotAndRejectsStaleTurns(t *testing.T) {
	a := pluginFixture(t, echoExtension)
	a.Config.Execution.Mode = "agent"
	a.environmentCommand("set TARGET=first.example", false)
	a.popup = nil
	a.proposeExtension("inspect", []byte(`{"message":"wraith::TARGET"}`))
	if a.proposal == nil {
		t.Fatal(a.notice)
	}
	p := *a.proposal
	a.environmentCommand("set TARGET=second.example", false)
	if !strings.Contains(string(p.Extension.Arguments), "first.example") || strings.Contains(string(p.Extension.Arguments), "second.example") {
		t.Fatal("approval arguments followed a later private variable change")
	}
	a.generation = "later-turn"
	a.runExtension(p)
	if len(a.jobs.Snapshot()) != 0 || a.proposal != nil {
		t.Fatal("old approval executed in another turn")
	}
}

func TestExtensionRemovedDefinitionExpiresApproval(t *testing.T) {
	a := pluginFixture(t, echoExtension+`wraith.commands.create("remove",function() wraith.tools.unregister("inspect") end)`)
	a.Config.Execution.Mode = "agent"
	a.proposeExtension("inspect", []byte(`{"message":"value"}`))
	applyPluginEvents(t, a, a.callPlugin(a.pluginManifest.Commands[0].Callback, "", ""))
	a.approvalAction("run")
	if a.proposal != nil || a.popup != nil || len(a.jobs.Snapshot()) != 0 || !strings.Contains(a.notice, "unavailable") {
		t.Fatalf("removed tool retained approval or misleading status: %s", a.notice)
	}
}
