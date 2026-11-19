package plugin

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestManagerProcessUsesControlledEnvironmentAndLiteralArguments inspects a real child instead of only launch options.
func TestManagerProcessUsesControlledEnvironmentAndLiteralArguments(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "ext::untrusted")
	t.Setenv("GIT_DIR", "/unexpected/repository")
	path, err := filepath.Abs("lua/wraith/manager/store_process.lua")
	if err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, fmt.Sprintf(`local process = dofile(%q)
assert(not pcall(process.environment,{GIT_CONFIG_GLOBAL="/unexpected"}))
assert(not pcall(process.environment,{GIT_DIR="/unexpected"}))
process.start({"/bin/sh","-c",'test -z "$GIT_CONFIG_COUNT$GIT_DIR" && test "$GIT_CONFIG_GLOBAL" = /dev/null && test "$GIT_TERMINAL_PROMPT" = 0 && printf "%%s" "$1" && printf diagnostic >&2',"manager","literal $(false); 'quoted'"}, {}, function(result)
 assert(result.state=="completed" and result.exit_code==0)
 assert(result.stdout=="literal $(false); 'quoted'" and result.stderr=="diagnostic")
 assert(not result.truncated)
 wraith.notify("complete")
end)`, path))
	runManagerProcess(t, r)
}

// TestManagerProcessWaitsForReapingAfterCancellation keeps cleanup behind actual worker completion.
func TestManagerProcessWaitsForReapingAfterCancellation(t *testing.T) {
	path, err := filepath.Abs("lua/wraith/manager/store_process.lua")
	if err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, fmt.Sprintf(`local process = dofile(%q)
local operation
operation=process.start({"/bin/sh","-c","printf ready; sleep 10"},{},function(result)
 assert(result.state=="cancelled")
 assert(wraith.resource.status(operation.id).done)
 wraith.notify("complete")
end)
local timer
timer=wraith.timer.start(10,function()
 if operation.output("stdout").data=="ready" then
  wraith.resource.cancel(timer)
  wraith.resource.release(timer)
  operation.cancel()
 end
end,{repeat_ms=10})`, path))
	runManagerProcess(t, r)
}

// runManagerProcess delivers actual managed events and applies the public lifecycle effects.
func runManagerProcess(t *testing.T, r *Runtime) {
	t.Helper()
	manifest := r.Manifest()
	if err := r.Activate(manifest.Resources); err != nil {
		t.Fatal(err)
	}
	for {
		event := receiveResource(t, r)
		if !r.ResourceValid(event) {
			continue
		}
		pending, err := r.SubmitResource(event, nil, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		complete := false
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			case "notify":
				complete = effect.Argument == "complete"
			default:
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
		if complete {
			r.services.mu.Lock()
			remaining := len(r.services.records)
			r.services.mu.Unlock()
			if remaining != 0 {
				t.Fatal("completed package process retained handles")
			}
			return
		}
	}
}

// TestManagerProcessRetainsBoundedFailureOutput keeps diagnostics without accepting truncated protocol data.
func TestManagerProcessRetainsBoundedFailureOutput(t *testing.T) {
	path, err := filepath.Abs("lua/wraith/manager/store_process.lua")
	if err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, fmt.Sprintf(`local process = dofile(%q)
process.start({"/bin/sh","-c","printf '%%070000d' 1; printf failure >&2; exit 7"},{},function(result)
 assert(result.state=="failed" and result.exit_code==7)
 assert(result.truncated and #result.stdout==65536 and result.stdout:sub(-1)=="1")
 assert(result.stderr=="failure" and result.error)
 wraith.notify("complete")
end)`, path))
	runManagerProcess(t, r)
}
