package plugin

import (
	"github.com/erdajt/wraith/internal/modeltool"
	"testing"
)

func TestProviderReplacementRestoresCoveredOwnerOnUnload(t *testing.T) {
	r := openTestRuntime(t, `
package.preload.first=function() return {setup=function()
 wraith.providers.register("custom",{description="first",models=function() end,show=function() end,chat=function() end})
end} end
package.preload.second=function() return {setup=function()
 wraith.providers.register("custom",{description="second",replace=true,models=function() end,show=function() end,chat=function() end})
end} end
wraith.plugins.load("first")
wraith.plugins.load("second")
wraith.commands.create("remove",function() wraith.plugins.unload("second") end)
`)
	original := r.Manifest()
	if len(original.Providers) != 1 || original.Providers[0].Description != "second" || len(original.Callbacks) != 7 {
		t.Fatalf("covered callbacks lost: %+v", original)
	}
	pending, err := r.Submit(original.Commands[0].Callback, "", nil, original.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Manifest == nil || result.Manifest.Providers[0].Description != "first" {
		t.Fatalf("owner restoration failed: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
	if len(r.Manifest().Callbacks) != 4 {
		t.Fatal("unloaded provider retained its callbacks")
	}
}

func TestToolRegistrationValidatesSchemasBeforeRetainingCallbacks(t *testing.T) {
	r := openTestRuntime(t, `
for i=1,600 do
 assert(not pcall(wraith.tools.register,"invalid",{parameters={type="object",['$ref']='#'},run=function() end}))
end
assert(not pcall(wraith.tools.register,"request_command",{parameters={type="object"},run=function() end}))
assert(not pcall(wraith.tools.register,"bad",{parameters={type="object"},approval="never",run=function() end}))
wraith.tools.register("local_status",{description="local status",parameters={type="object",properties={limit={type="integer",minimum=1}}},run=function() end})
wraith.commands.create("failed",function()
 wraith.tools.unregister("local_status")
 error("rollback")
end)
`)
	manifest := r.Manifest()
	if len(manifest.Tools) != 1 || manifest.Tools[0].Approval != "required" || len(manifest.Callbacks) != 2 {
		t.Fatalf("invalid tools retained registrations: %+v", manifest)
	}
	pending, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	if result := receiveResult(t, pending); result.Err == nil {
		t.Fatal("failed tool removal succeeded")
	}
	if len(r.Manifest().Tools) != 1 {
		t.Fatal("tool removal escaped rollback")
	}
	manifest.Tools[0].Parameters[0] = 'x'
	if r.Manifest().Tools[0].Parameters[0] != '{' {
		t.Fatal("manifest retained mutable tool schema")
	}
}

func TestToolSchemasPreserveExplicitJSONContainersAndNull(t *testing.T) {
	r := openTestRuntime(t, `
wraith.json.encode=function() error("must not invoke mutable public codec") end
wraith.tools.register("containers",{
 parameters={type="object",required=wraith.json.array({}),properties={
  empty_array={const=wraith.json.array({})},empty_object={const=wraith.json.object({})},nullable={enum={wraith.json.null,"value"}}
 }},run=function() end
})
`)
	schema, err := modeltool.Compile(r.Manifest().Tools[0].Parameters)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"empty_array":[],"empty_object":{},"nullable":null}`)); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{"empty_array":{},"empty_object":[],"nullable":false}`)); err == nil {
		t.Fatal("schema lost explicit container types")
	}
}
