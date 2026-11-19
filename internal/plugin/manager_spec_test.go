package plugin

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestManagerDescriptorsDetachSourcesAndRejectUnsafeRefs(t *testing.T) {
	path, err := filepath.Abs("lua/wraith/manager/spec.lua")
	if err != nil {
		t.Fatal(err)
	}
	openTestRuntime(t, fmt.Sprintf(`local spec=dofile(%q)
 local input={name="app",source={kind="git",url="https://example.com/app"},selector={kind="branch",value="feature/status"},dependencies={"lib"}}
 local result=spec.descriptor(input)
 input.source.url="https://changed.example/app"; input.dependencies[1]="changed"
 assert(result.source.url=="https://example.com/app" and result.dependencies[1]=="lib")
 for _,url in ipairs({"https://token@example.com/app","https://example.com/app?token=secret","ext::program","-remote","relative/path","ssh://user:password@example.com/app"}) do
  input.source.url=url; assert(not pcall(spec.descriptor,input),url)
 end
 for _,url in ipairs({"git@example.com:owner/app","ssh://git@example.com/owner/app","file:///tmp/repository","/tmp/local repository"}) do
  input.source.url=url; assert(spec.descriptor(input).source.url==url)
 end
 input.source.url="https://example.com/app"
 for _,ref in ipairs({"-option","a..b","a@{b","a.lock","a/.hidden","a//b","a/b.","a\\b","a b","a?b"}) do
  input.selector.value=ref; assert(not pcall(spec.descriptor,input),ref)
 end
 input.selector={kind="commit",value=string.rep("A",40)}
 assert(spec.descriptor(input).selector.value==string.rep("a",40))
 input.selector.value="abc123"; assert(not pcall(spec.descriptor,input))
 input.source={kind="local",dir="/explicit/package"}; input.selector={kind="head"}
 assert(spec.descriptor(input).source.dir=="/explicit/package")
 input.pin=true; assert(not pcall(spec.descriptor,input))
 input.pin=false; input.dependencies={[1]="lib",extra="ignored"}; assert(not pcall(spec.descriptor,input))
 `, path))
}
