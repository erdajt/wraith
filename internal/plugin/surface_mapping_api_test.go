package plugin

import (
	"slices"
	"testing"
)

func TestBufferCallbackPublicationAndOwnedCopies(t *testing.T) {
	r := openTestRuntime(t, `
local b = wraith.buffers.create({lines={"shared"}})
package.preload.maps = function() return {setup=function()
 wraith.buffers.set_keymap(b,"buffer.normal","x",function() wraith.notify("mapped") end)
end} end
wraith.plugins.load("maps")
local action = wraith.buffers.get(b).keymaps[1].action
wraith.buffers.set_keymap(b,"buffer.normal","y",action)
wraith.commands.create("unload",function() wraith.plugins.unload("maps") end)
`)
	manifest := r.Manifest()
	for _, mutation := range manifest.Surface {
		if _, err := r.Surfaces().Apply(mutation); err != nil {
			t.Fatal(err)
		}
	}
	id := manifest.Surface[0].ID
	buffer, _ := r.Surfaces().Buffer(id)
	if len(buffer.Keymaps) != 2 || !slices.Contains(manifest.Callbacks, buffer.Keymaps[0].Action) {
		t.Fatalf("mapping callback absent from manifest: %+v", buffer.Keymaps)
	}
	result, err := r.Submit(buffer.Keymaps[0].Action, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, result)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "mapped" {
		t.Fatalf("mapping invocation: %+v", got)
	}
	result, err = r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, result)
	if got.Err != nil || got.Manifest == nil || r.Acknowledge(got.Revision, true) != nil {
		t.Fatalf("unload: %+v", got)
	}
	buffer, _ = r.Surfaces().Buffer(id)
	if len(buffer.Keymaps) != 0 {
		t.Fatalf("copied unloaded callback survived: %+v", buffer.Keymaps)
	}
}

func TestRejectedBufferCallbackRegistrationReclaimsOwnedState(t *testing.T) {
	r := openTestRuntime(t, `
local b = wraith.buffers.create({lines={"shared"}})
wraith.commands.create("map",function()
 assert(not pcall(wraith.buffers.set_keymap,b,"buffer.normal","x",function() end,"",99))
 wraith.buffers.set_keymap(b,"buffer.normal","y",function() wraith.notify("child") end)
end)
`)
	manifest := r.Manifest()
	_, _ = r.Surfaces().Apply(manifest.Surface[0])
	result, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, result)
	if got.Err != nil || got.Manifest == nil || len(got.Manifest.Callbacks) != len(manifest.Callbacks)+1 {
		t.Fatalf("caught failure leaked a callback: %+v", got)
	}
	if err := r.Acknowledge(got.Revision, false); err != nil {
		t.Fatal(err)
	}
	buffer, _ := r.Surfaces().Buffer(manifest.Surface[0].ID)
	if len(buffer.Keymaps) != 0 || len(r.Manifest().Callbacks) != len(manifest.Callbacks) {
		t.Fatal("rejected mapping registration became visible")
	}
}

func TestStartupUnloadDropsCopiedSurfaceCallbackTargets(t *testing.T) {
	r := openTestRuntime(t, `
local b = wraith.buffers.create({lines={"shared"}})
package.preload.maps = function() return {setup=function()
 wraith.keymap.set("prompt.normal","x",function() error("unloaded callback") end)
end} end
wraith.plugins.load("maps")
for _, mapping in ipairs(wraith.config.get("keymaps")) do
 if mapping.keys == "x" then wraith.buffers.set_keymap(b,"buffer.normal","y",mapping.action) end
end
wraith.plugins.unload("maps")
assert(#wraith.buffers.get(b).keymaps == 0)
`)
	manifest := r.Manifest()
	if len(manifest.Surface) != 1 || len(manifest.Callbacks) != 0 {
		t.Fatalf("unloaded startup target survived: surfaces=%+v callbacks=%+v", manifest.Surface, manifest.Callbacks)
	}
	if _, err := r.Surfaces().Apply(manifest.Surface[0]); err != nil {
		t.Fatal(err)
	}
}
