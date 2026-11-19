package plugin

import (
	"fmt"
	"os"
	"testing"
)

func TestLuaProcessExchangesFramedMessages(t *testing.T) {
	source := fmt.Sprintf(`local bytes = ""
local payload = '{"text":"界"}'
local frame = "Content-Length: " .. #payload .. "\r\n\r\n" .. payload
wraith.process.start({argv={%q, "-test.run=^TestManagedProcessFixture$", "--", "frames"},
 env={WRAITH_PROCESS_FIXTURE="1"}, stdin=true, mode="protocol"}, function(ctx)
 local id = ctx.resource.id
 if ctx.resource.kind == "started" then
  assert(wraith.process.write(id, frame:sub(1, 5)) == 5)
  wraith.process.write(id, frame:sub(6) .. frame)
  wraith.process.close_stdin(id)
 elseif ctx.resource.kind == "stdout" then
  bytes = bytes .. ctx.data.data
 elseif ctx.resource.final then
  assert(ctx.data.state == "completed" and ctx.data.result.exit_code == 0)
  assert(bytes == frame .. frame)
  assert(wraith.process.output(id).data == bytes)
  assert(wraith.resource.status(id).done)
  wraith.resource.release(id)
  wraith.notify("complete")
 end
end)`, os.Args[0])
	runtime := openTestRuntime(t, source)
	manifest := runtime.Manifest()
	if err := runtime.Activate(manifest.Resources); err != nil {
		t.Fatal(err)
	}
	for {
		event := receiveResource(t, runtime)
		pending, err := runtime.SubmitResource(event, map[string]any{}, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if event.Final {
			if len(result.Effects) != 2 || result.Effects[0].Name != "resource.release" || result.Effects[1].Argument != "complete" {
				t.Fatalf("framed exchange did not finish: %+v", result)
			}
			runtime.DiscardResources([]string{result.Effects[0].Argument})
			if runtime.ResourceValid(event) {
				t.Fatal("released process remained live")
			}
			return
		}
	}
}

func TestLuaProcessRejectsInvalidLaunchAndQueryOptions(t *testing.T) {
	openTestRuntime(t, `for _, options in ipairs({
 {}, {argv={}}, {argv={""}}, {argv={"echo", [3]="hole"}}, {argv={1}},
 {argv={"echo"}, shell="echo"}, {argv={"echo"}, shell=""},
 {shell="echo", shell_path="relative"}, {argv={"echo"}, shell_path=""},
 {argv={"echo"}, mode="unknown"}, {argv={"echo"}, stdin="yes"},
 {argv={"echo"}, env={BAD=true}}, {argv={"echo"}, env={["BAD=KEY"]="x"}},
 {argv={"echo"}, timeout_ms=0/0}, {argv={"echo"}, max_output_bytes=0},
 {argv={"echo"}, cwd=""}, {argv={"echo"}, unknown=true}
}) do
 assert(not pcall(wraith.process.start, options, function() end))
end
local id = wraith.process.start({shell="printf sample", clear_env=true, env={KEEP="value", DROP=false}}, function() end)
assert(not pcall(wraith.process.write, id, "before start"))
assert(not pcall(wraith.process.output, id, {stream="combined"}))
assert(not pcall(wraith.process.output, id, {max_bytes=0}))
assert(not pcall(wraith.process.output, id, {offset=0/0}))
assert(not pcall(wraith.process.output, id, {unknown=true}))
assert(not pcall(wraith.process.output, "foreign"))
wraith.resource.release(id)`)
}
