package ui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/runner"
	"github.com/erdajt/wraith/internal/variables"
)

const echoExtension = `wraith.tools.register("inspect",{
 description="inspect supplied text",parameters={type="object",additionalProperties=false,required={"message"},properties={message={type="string"}}},
 run=function(ctx)
  local id=ctx.invocation.id
  assert(ctx.data.job_id and ctx.data.directory)
  wraith.invocation.emit(id,{progress="working"})
  wraith.schedule(function() wraith.invocation.finish(id,{message=ctx.data.arguments.message}) end)
 end
})`

func TestExtensionApprovalRunsOnceInBackgroundWithPrivateOutputControl(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(map[bool]string{false: "shared", true: "hidden"}[hidden], func(t *testing.T) {
			a := pluginFixture(t, echoExtension)
			a.Config.Execution.Mode = "agent"
			a.Prompt.Set("draft stays available")
			a.proposeExtension("inspect", []byte(`{"message":"original"}`))
			if a.proposal == nil || a.popup == nil || len(a.jobs.Snapshot()) != 0 {
				t.Fatal("tool skipped approval")
			}
			id := a.proposal.launch.ID
			a.proposal.HideOutput = hidden
			if a.approvalAction("edit") == nil || a.editorFile == nil {
				t.Fatal("tool editor did not open")
			}
			file := a.editorFile
			if err := os.WriteFile(file.Path, []byte(`{"message":"edited"}`), 0600); err != nil {
				t.Fatal(err)
			}
			a.finishEdit(editedMsg{Kind: "extension-tool", File: file})
			if a.proposal != nil || a.busy || a.Prompt.String() != "draft stays available" {
				t.Fatal("tool blocked input or requested a second approval")
			}
			err := runTransport(t, a, func() error {
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				for {
					for _, job := range a.jobs.Snapshot() {
						if !job.Record.Finished.IsZero() {
							return job.Err
						}
					}
					select {
					case <-a.jobs.Updates:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			record, err := a.history().Load(id)
			if err != nil || record.Status != "completed" || !strings.Contains(record.Output, `"message":"edited"`) || record.OutputHidden != hidden || record.ShareOutput == hidden {
				t.Fatalf("tool record: %+v, %v", record, err)
			}
			_, err = runner.ReadOutput(record, runner.Query{}, variables.Snapshot{})
			if (err != nil) != hidden {
				t.Fatalf("tool output permission: %v", err)
			}
			evidence := a.commandEvidence(record)
			if evidence.Hidden != hidden || hidden && evidence.Output != "" {
				t.Fatalf("tool evidence leaked: %+v", evidence)
			}
		})
	}
}

func TestExtensionDenialAndInvalidEditsCannotExecute(t *testing.T) {
	a := pluginFixture(t, echoExtension)
	a.Config.Execution.Mode = "agent"
	a.proposeExtension("inspect", []byte(`{"message":"original"}`))
	a.approvalAction("edit")
	file := a.editorFile
	if err := os.WriteFile(file.Path, []byte(`{"message":12}`), 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "extension-tool", File: file})
	if len(a.jobs.Snapshot()) != 0 || a.proposal == nil {
		t.Fatal("invalid arguments executed or lost their approval")
	}
	a.approvalAction("cancel")
	if !a.agentStopped || a.proposal != nil || len(a.jobs.Snapshot()) != 0 {
		t.Fatal("denial did not stop the tool turn")
	}
}

func TestExtensionDefinitionReplacementRequiresAnotherApproval(t *testing.T) {
	a := pluginFixture(t, echoExtension+`
wraith.commands.create("replace",function()
 wraith.tools.register("inspect",{replace=true,description="changed",parameters={type="object"},run=function() error("must not execute") end})
end)`)
	a.Config.Execution.Mode = "agent"
	a.proposeExtension("inspect", []byte(`{"message":"original"}`))
	original := a.proposal.launch.ID
	applyPluginEvents(t, a, a.callPlugin(a.pluginManifest.Commands[0].Callback, "", ""))
	a.approvalAction("run")
	if a.proposal == nil || a.proposal.launch.ID == original || a.proposal.Reason != "changed" || len(a.jobs.Snapshot()) != 0 {
		t.Fatalf("replacement reused approval: %+v", a.proposal)
	}
}
