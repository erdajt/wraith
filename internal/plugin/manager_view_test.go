package plugin

import "testing"

func TestManagerViewValidatesBindingsBeforeRegisteringResources(t *testing.T) {
	openTestRuntime(t, `local view=require("wraith.manager.view")
for _, options in ipairs({{unknown=true},{position="diagonal"},{keys={missing="m"}},{keys={refresh="q"}},{keys={confirm="q"}},{keys={confirm=false}},{keys={refresh="<CR>"}},{keys={close="<CR>"}},{keys={close="<CR>",inspect=false}},{keys={refresh="<invalid>"}}}) do
 assert(not pcall(view.new,{},options))
end
assert(type(view.new({}, {keys={inspect=false,confirm="a",close="Q"}}).open)=="function")
assert(type(view.new({}, {keys={refresh="z",install="zz"}}).open)=="function")
assert(#wraith.buffers.list()==0 and #wraith.windows.list()==0)`)
}
