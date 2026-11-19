package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandPolicyTransformsBeforeApprovalAndRechecksBeforeLaunch(t *testing.T) {
	a := pluginFixture(t, `
local executions = 0
wraith.hooks.add("before_command", function(ctx)
  if ctx.phase == "preview" then ctx.command.command = "printf preview" end
  if ctx.phase == "execute" then
    executions = executions + 1
    if executions == 1 then ctx.command.command = "printf revised" end
  end
  return {command=ctx.command}
end)`)
	cmd := a.approve(proposal{Command: "printf original"})
	if cmd == nil || a.popup.Kind != "command-policy" || len(a.jobs.Snapshot()) != 0 {
		t.Fatal("preview failed to wait for policy")
	}
	a.Update(cmd())
	if a.popup.Kind != "approval" || a.proposal.Resolved != "printf preview" {
		t.Fatal("approval did not show transformed command")
	}
	a.Update(a.runApproved()())
	if a.popup.Kind != "approval" || a.proposal.Resolved != "printf revised" || len(a.jobs.Snapshot()) != 0 {
		t.Fatal("execute transformation bypassed renewed approval")
	}
	a.Update(a.runApproved()())
	jobs := a.jobs.Snapshot()
	if len(jobs) != 1 || jobs[0].Record.Command != "printf revised" {
		t.Fatalf("final approved tuple was not launched: %+v", jobs)
	}
}

func TestEditedPolicyCommandRunsOnceUnlessPolicyChangesSavedText(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "changed"}[change], func(t *testing.T) {
			source := `wraith.hooks.add("before_command", function(ctx) return {command=ctx.command} end)`
			if change {
				source = `wraith.hooks.add("before_command", function(ctx)
  if ctx.command.command == "printf edited" then ctx.command.command = "printf changed" end
  return {command=ctx.command}
end)`
			}
			a := pluginFixture(t, source)
			a.Update(a.approve(proposal{Command: "printf original"})())
			a.proposal.Command, a.proposal.Resolved, a.proposal.policyEdited = "printf edited", "printf edited", true
			_, next := a.Update(a.runApproved()())
			if change {
				if next != nil || a.popup.Kind != "approval" || a.proposal.Resolved != "printf changed" || len(a.jobs.Snapshot()) != 0 {
					t.Fatal("edited preview transformation ran without approval")
				}
				return
			}
			if next == nil {
				t.Fatal("saved edit did not reach execution recheck")
			}
			a.Update(next())
			if jobs := a.jobs.Snapshot(); len(jobs) != 1 || jobs[0].Record.Command != "printf edited" {
				t.Fatalf("unchanged saved edit did not run once: %+v", jobs)
			}
		})
	}
}

func TestCommandPolicyRejectsDenialCancellationAndInvalidTuple(t *testing.T) {
	for _, failure := range []string{"deny", "invalid", "cancel", "session"} {
		t.Run(failure, func(t *testing.T) {
			source := `wraith.hooks.add("before_command", function(ctx) return {command=ctx.command} end)`
			if failure == "deny" {
				source = `wraith.hooks.add("before_command", function() return {decision="deny",reason="outside scope"} end)`
			}
			if failure == "invalid" {
				source = `wraith.hooks.add("before_command", function(ctx) ctx.command.directory="relative"; return {command=ctx.command} end)`
			}
			a := pluginFixture(t, source)
			cmd := a.approve(proposal{Command: "printf blocked", Continue: true})
			result := cmd()
			if failure == "cancel" {
				a.menuKey(tea.KeyMsg{Type: tea.KeyEsc})
			}
			if failure == "session" {
				a.Session.ID = "another-session"
			}
			a.Update(result)
			if len(a.jobs.Snapshot()) != 0 || a.popup != nil && a.popup.Kind == "approval" {
				t.Fatal("failed or stale check became actionable")
			}
			if failure == "deny" && !strings.Contains(a.notice, "outside scope") {
				t.Fatal("policy denial reason was hidden")
			}
		})
	}
}

func TestPolicyRemovalInvalidatesBothOpenAndPendingApproval(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "open", true: "pending"}[pending], func(t *testing.T) {
			a := pluginFixture(t, `
local hook = wraith.hooks.add("before_command", function(ctx) return {command=ctx.command} end)
wraith.commands.create("removehook", function() wraith.hooks.remove(hook) end)`)
			a.Update(a.approve(proposal{Command: "printf approved"})())
			var result tea.Msg
			if pending {
				result = a.runApproved()()
			}
			applyPluginEvents(t, a, a.callPlugin(a.pluginManifest.Commands[0].Callback, "", ""))
			if pending {
				a.Update(result)
			} else {
				a.runApproved()
			}
			if len(a.jobs.Snapshot()) != 0 || a.popup == nil || a.popup.Kind != "approval" {
				t.Fatalf("removing policy left jobs=%d popup=%+v hooks=%d notice=%s", len(a.jobs.Snapshot()), a.popup, len(a.pluginManifest.Hooks), a.notice)
			}
			a.runApproved()
			if len(a.jobs.Snapshot()) != 1 {
				t.Fatal("fresh approval did not permit execution under the new policy")
			}
		})
	}
}

func TestPolicyLaunchFailureRestoresActionableApproval(t *testing.T) {
	a := pluginFixture(t, `wraith.hooks.add("before_command", function(ctx) return {command=ctx.command} end)`)
	a.Update(a.approve(proposal{Command: "printf approved"})())
	a.jobs.Close()
	a.Update(a.runApproved()())
	if a.popup == nil || a.popup.Kind != "approval" || a.proposal == nil || !strings.Contains(a.notice, "command launch") {
		t.Fatalf("launch failure stranded policy popup: %s", a.notice)
	}
}
