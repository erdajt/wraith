package plugin

import (
	"fmt"
	"strings"
	"testing"
)

// interrupted entry loads preserve native cache values and require a runtime reload.
func TestManagerEntryBindingRequiresReloadAfterCallbackDeadline(t *testing.T) {
	for _, tc := range []struct {
		name, value, write string
		direct             bool
	}{
		{name: "native marker", value: "nil"},
		{name: "ordinary require after interruption", value: "nil", direct: true},
		{name: "explicit nil", value: "nil", write: "cache.interrupted_entry=retained"},
		{name: "false", value: "false", write: "cache.interrupted_entry=retained"},
		{name: "table", value: "{}", write: "cache.interrupted_entry=retained"},
		{name: "ordinary require with retained cache", value: "{}", write: "cache.interrupted_entry=retained", direct: true},
		{name: "userdata", value: "newproxy()", write: "cache.interrupted_entry=retained"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := openManagerRuntime(t, fmt.Sprintf(`
local bindings=require("wraith.manager.loader_binding")
local cache,retained,calls=package.loaded,%s,0
local source=function()
 calls=calls+1
 %s
 while true do end
end
package.preload.interrupted_entry=source
local owned=bindings.new({name="interrupted_entry",kind="preload",identity=source,load=source})
owned.install()
wraith.commands.create("timeout",function() owned.require() end)
wraith.commands.create("verify",function()
 assert(calls==1)
 assert(rawequal(cache.interrupted_entry,retained))
 if %t then
  local ok,result=pcall(require,"interrupted_entry")
  if retained then
   assert(ok and rawequal(result,retained))
  else
   assert(not ok and tostring(result):find("interrupted",1,true) and tostring(result):find("/reload",1,true),tostring(result))
  end
  assert(calls==1 and rawequal(cache.interrupted_entry,retained))
 end
 local ok,err=pcall(owned.require)
 assert(not ok and tostring(err):find("interrupted",1,true) and tostring(err):find("/reload",1,true),tostring(err))
 assert(calls==1 and rawequal(cache.interrupted_entry,retained))
 cache.interrupted_entry=nil
 ok,err=pcall(owned.require)
 assert(not ok and tostring(err):find("/reload",1,true),tostring(err))
 assert(calls==1 and cache.interrupted_entry==nil)
end)
`, tc.value, tc.write, tc.direct))
			manifest := r.Manifest()
			var callback string
			for _, command := range manifest.Commands {
				if command.Name == "timeout" {
					callback = command.Callback
				}
			}
			if callback == "" {
				t.Fatal("timeout command is missing")
			}
			pending, err := r.Submit(callback, "", nil, manifest.Config)
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			if result.Err == nil || !strings.Contains(result.Err.Error(), "context deadline exceeded") {
				t.Fatalf("entry did not reach the callback deadline: %v", result.Err)
			}
			if result.Manifest != nil || len(result.Effects) != 0 {
				t.Fatalf("interrupted entry published changes: %+v", result)
			}

			managerCommand(t, r, "verify", "")
		})
	}
}
