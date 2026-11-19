package plugin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSurfaceStartupCommitReleasesLogsBeforeFirstPublication(t *testing.T) {
	r := openTestRuntime(t, `
local b = wraith.buffers.create({lines={"retained text"}})
wraith.commands.create("extend",function()
 assert(wraith.buffers.get(b).lines[1] == "retained text")
 wraith.commands.create("child",function() end)
end)
`)
	manifest := r.Manifest()
	for _, mutation := range manifest.Surface {
		if _, err := r.Surfaces().Apply(mutation); err != nil {
			t.Fatal(err)
		}
	}
	r.CommitStartup()
	r.CommitStartup()
	if len(r.Manifest().Surface) != 0 {
		t.Fatal("host retained installed startup text")
	}
	pending, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, pending)
	if got.Err != nil || got.Manifest == nil || len(got.Manifest.Surface) != 0 {
		t.Fatalf("actor republished consumed startup log: %+v", got)
	}
	if err := r.Acknowledge(got.Revision, true); err != nil {
		t.Fatal(err)
	}
	if len(r.Manifest().Surface) != 0 || len(r.Surfaces().Index().Buffers) != 1 {
		t.Fatal("startup consumption removed live text or retained its mutation log")
	}
}

func TestSurfaceStartupBudgetRejectsRepeatedRetainedPayloads(t *testing.T) {
	r := openTestRuntime(t, `
local line = string.rep("x", 60000)
local lines = {line,line,line,line,line,line,line,line}
local b = wraith.buffers.create({lines=lines})
local rejected = false
for i=1,25 do
 local ok,err = pcall(wraith.buffers.set_lines,b,0,-1,lines)
 if not ok then
  assert(tostring(err):find("8 mib",1,true))
  rejected = true
  break
 end
end
assert(rejected)
assert(wraith.buffers.get_lines(b,0,1).lines[1] == line)
`)
	manifest := r.Manifest()
	encoded, err := json.Marshal(manifest.Surface)
	if err != nil || len(encoded) > 8<<20 || len(manifest.Surface) >= 128 {
		t.Fatalf("startup payload budget failed: bytes=%d operations=%d err=%v", len(encoded), len(manifest.Surface), err)
	}
}

func TestSurfaceStartupCommitReleasesIdleActorState(t *testing.T) {
	r := openTestRuntime(t, `wraith.buffers.create({lines={string.rep("x",60000)}})`)
	for _, mutation := range r.Manifest().Surface {
		if _, err := r.Surfaces().Apply(mutation); err != nil {
			t.Fatal(err)
		}
	}
	r.CommitStartup()
	select {
	case <-r.startupConsumed:
	case <-time.After(time.Second):
		t.Fatal("idle actor did not consume installed startup state")
	}
	if len(r.Manifest().Surface) != 0 || len(r.Surfaces().Index().Buffers) != 1 {
		t.Fatal("startup consumption lost live text or retained bootstrap mutations")
	}
}
