package plugin

import (
	"fmt"
	"testing"
)

func TestManagerDescriptorsDetachSourcesAndRejectUnsafeRefs(t *testing.T) {
	path := managerModule(t, "spec.lua")
	openManagerRuntime(t, fmt.Sprintf(`local spec=dofile(%q)
 local input={name="app",source={kind="git",url="https://example.com/app"},selector={kind="branch",value="feature/status"},dependencies={"lib"}}
 local result=spec.descriptor(input)
 input.source.url="https://changed.example/app"; input.dependencies[1]="changed"
 assert(result.source.url=="https://example.com/app" and result.dependencies[1]=="lib")
 for _,url in ipairs({"https://token@example.com/app","https://example.com/app?token=secret","http://example.com/app","git@-host:repo","ssh://-host/repo","git@host:-path","host:-path","ext::program","-remote","relative/path","ssh://user:password@example.com/app"}) do
  input.source.url=url; assert(not pcall(spec.descriptor,input),url)
 end
 for _,url in ipairs({"example.com:owner/app","git@example.com:owner/app","ssh://git@example.com/owner/app","file:///tmp/repository","/tmp/local repository"}) do
  input.source.url=url; assert(spec.descriptor(input).source.url==url)
 end
 input.source.url="https://example.com/app"
 for _,ref in ipairs({"-option","a..b","a@{b","a.lock","a/.hidden","a//b","a/b.","a\\b","a b","a?b",string.rep("a",256)}) do
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

func TestManagerSpecsResolveNestedImportsAndDependencyOrder(t *testing.T) {
	openManagerRuntime(t, `local spec=require("wraith.manager.spec")
 package.preload["settings.tools"]=function() return {{"owner/app",dependencies={"library"},cmd="app",opts={nested={value=1}}}} end
 local enabled=0
 local input={{{name="library",dir="/tmp/library",priority=-10}}, {import="settings.tools"},
  {"owner/first",priority=20,enabled=function() enabled=enabled+1; return true end},
  {"owner/disabled",enabled=false}, {"owner/key",keys={{"z",function() end,desc="inspect"}}}}
 local graph=spec.normalize(input)
 assert(enabled==1 and #graph.ordered==4)
 assert(graph.ordered[1].name=="first" and graph.ordered[2].name=="library" and graph.ordered[3].name=="app")
 assert(graph.by_name.app.lazy and not graph.by_name.library.lazy)
 assert(graph.by_name.key.keys[1].mode=="chat.normal" and graph.by_name.key.keys[1].description=="inspect")
 input[1][1].dir="/changed"
 assert(graph.by_name.library.source.dir=="/tmp/library")
 assert(graph.by_name.app.source.url=="https://github.com/owner/app.git")
 local relative=spec.normalize({{name="local",dir="./packages/../local"}})
 assert(relative.by_name["local"].source.dir==wraith.paths.stdpath("config").."/local")
 local dependency=spec.normalize({{"owner/lazy",lazy=true,dependencies={{"owner/dependency"}}}})
 assert(not dependency.by_name.dependency.lazy and dependency.by_name.lazy.lazy)
 `)
}

func TestManagerSpecsRejectInvalidGraphsBeforeInitialization(t *testing.T) {
	openManagerRuntime(t, `local spec=require("wraith.manager.spec")
 local initialized=0
 local function rejects(value, expected)
  local ok,err=pcall(spec.normalize,value)
  assert(not ok and tostring(err):find(expected,1,true), tostring(err))
  assert(initialized==0)
 end
 rejects({{"owner/app",init=function() initialized=initialized+1 end},{"other/app"}},"conflicting source")
 rejects({{"owner/app",dependencies={"missing"}}},"unknown dependency")
 rejects({{"owner/app",dependencies={"lib"}},{"owner/lib",dependencies={"app"}}},"dependency cycle")
 rejects({{"owner/app",dependencies={"lib"}},{"owner/lib",enabled=false}},"depends on disabled")
 rejects({{"owner/app",main="shared"},{"owner/lib",main="shared"}},"duplicate package main")
 rejects({{"owner/app",cmd="same"},{"owner/lib",cmd="same"}},"duplicate command trigger")
 rejects({{"owner/app",keys={{"x"}}}},"explicit action")
 rejects({{"owner/app",keys={{"<Enter>","command.help"},{"<CR>","command.help"}}}},"duplicate key trigger")
 assert(spec.normalize({{"owner/prefix",keys={{"z","command.help"},{"zz","command.help"}}}}))
 rejects({{"owner/app",unknown=true}},"unknown package field")
 rejects({{name="empty",dir=""}},"nonempty string")
 for _,field in ipairs({"name","main","priority","keys","dependencies"}) do
  local value={"owner/app"}; value[field]=false
  local ok=pcall(spec.normalize,{value}); assert(not ok,field)
 end
 rejects({{"owner/app",keys={{"x","command.help",mode=false}}}},"invalid key trigger mode")
 rejects({{"owner/app",keys={{"x","command.help",desc=false}}}},"invalid key trigger mode")
 assert(spec.normalize({{"owner/app",lazy=false,config=false,pin=false}}))
 rejects({{"owner/app",keys={{"x","command.help",mode="not-a-mode"}}}},"unknown mode")
 rejects({{"owner/app",keys={{"<InvalidKey>","command.help"}}}},"keymap.validate")
 rejects({{"owner/app",event="unknown_native_event"}},"unknown event")
 rejects({{"owner/app",branch="main",tag="v1"}},"choose one branch")
 package.preload["settings.loop"]=function() return {{import="settings.loop"}} end
 rejects({{import="settings.loop"}},"import cycle")
 local cyclic={}; cyclic[1]=cyclic; rejects(cyclic,"cyclic package")
 local many={}; for i=1,64 do many[i]={name="p"..i,dir="/tmp/p"..i} end
 rejects(many,"exceed 63")
 `)
}
