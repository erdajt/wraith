package plugin

import "testing"

func TestDetachedPackagesAcquireExplicitSharedDependencies(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.manager=function() return {setup=function() end} end
 package.preload.child=function() return {setup=function() wraith.plugins.depend("manager") end} end
 package.preload.parent=function() return {setup=function()
  wraith.plugins.load("child",{}, {dependency=false})
 end} end
 wraith.plugins.load("manager")
 wraith.plugins.load("parent")
 wraith.commands.create("verify",function()
  wraith.plugins.unload("child")
  wraith.plugins.unload("parent")
  wraith.plugins.unload("manager")
 end)
 `)
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil {
		t.Fatalf("detached child could not unload independently: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
}

func TestPackageDependencyAdmissionRejectsCyclesAndUninitializedTargets(t *testing.T) {
	openTestRuntime(t, `
 package.preload.first=function() return {setup=function()
  assert(not pcall(wraith.plugins.depend,"first"))
  assert(not pcall(wraith.plugins.depend,"missing"))
 end} end
 package.preload.second=function() return {setup=function()
  wraith.plugins.depend("first")
  wraith.plugins.depend("first")
 end} end
 wraith.plugins.load("first")
 wraith.plugins.load("second")
 assert(not pcall(wraith.plugins.depend,"first"))
 assert(not pcall(wraith.plugins.unload,"first"))
 assert(not pcall(wraith.plugins.load,"missing",{}, {dependency="false"}))
 `)
	e := &engine{packages: map[string]packageRecord{
		"first":  {owner: "a", dependencies: []string{"b"}},
		"second": {owner: "b"},
	}}
	if err := e.dependOnPackage("b", "first"); err == nil {
		t.Fatal("dependency cycle accepted")
	}
	if len(e.packages["second"].dependencies) != 0 {
		t.Fatal("rejected cycle changed dependency graph")
	}
}

func TestSharedDependenciesRemainUntilEveryDependentUnloads(t *testing.T) {
	openTestRuntime(t, `
 package.preload.shared=function() return {setup=function() end,link=function()
  wraith.plugins.depend("first")
 end} end
 package.preload.first=function() return {setup=function()
  wraith.plugins.depend("shared")
  wraith.plugins.depend("shared")
 end} end
 package.preload.second=function() return {setup=function() wraith.plugins.depend("shared") end} end
 wraith.plugins.load("shared")
 wraith.plugins.load("first")
 wraith.plugins.load("second")
 local ok,err=pcall(wraith.plugins.call,"shared","link")
 assert(not ok and string.find(err,"cycle"))
 wraith.plugins.unload("first")
 assert(not pcall(wraith.plugins.unload,"shared"))
 wraith.plugins.unload("second")
 wraith.plugins.unload("shared")
 `)
}
