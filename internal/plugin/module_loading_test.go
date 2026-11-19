package plugin

import (
	"fmt"
	"strings"
	"testing"
)

// TestModuleLoadingTracksNativeRequire separates native loads from direct preload calls and cached values.
func TestModuleLoadingTracksNativeRequire(t *testing.T) {
	e, _ := moduleFixture(t, nil)
	if err := e.L.DoString(`
assert(wraith.capabilities.module_loading)
local cache, calls = package.loaded, 0
package.preload.probe=function(name)
 calls=calls+1
 assert(wraith.modules.loading(name)==(calls~=1))
 return {call=calls}
end
assert(not wraith.modules.loading("probe"))
assert(package.preload.probe("probe").call==1)
assert(not wraith.modules.loading("probe") and cache.probe==nil)
assert(require("probe").call==2 and not wraith.modules.loading("probe"))
assert(require("probe").call==2 and calls==2)
cache.probe=nil
assert(require("probe").call==3)

local attempts=0
package.preload.retry=function(name)
 attempts=attempts+1
 assert(wraith.modules.loading(name))
 if attempts==1 then error("first load") end
 return true
end
assert(not pcall(require,"retry"))
assert(cache.retry==nil and not wraith.modules.loading("retry"))
assert(require("retry")==true and attempts==2)

package.preload.registry=function(name)
 package.loaded={}
 assert(wraith.modules.loading(name))
 package.loaded=cache
 cache[name]=false
 assert(not wraith.modules.loading(name))
end
assert(require("registry")==false)
`); err != nil {
		t.Fatal(err)
	}
}

// TestModuleLoadingRejectsInvalidNames keeps introspection within the public module grammar.
func TestModuleLoadingRejectsInvalidNames(t *testing.T) {
	e, _ := moduleFixture(t, nil)
	for _, name := range []string{"", "1bad", "part..name", "../name", "with space", strings.Repeat("a", 129)} {
		if err := e.L.DoString(fmt.Sprintf(`local ok,err=pcall(wraith.modules.loading,%q); assert(not ok and tostring(err):find("invalid module name",1,true))`, name)); err != nil {
			t.Fatal(err)
		}
	}
}
