wraith.notify(false) -- expect: param-type-mismatch
wraith.notify() -- expect: missing-parameter
wraith.paths.stdpath("unknown") -- expect: param-type-mismatch
wraith.input.set_mode("replace") -- expect: param-type-mismatch
wraith.keymap.set("wrong", "x", "quit") -- expect: param-type-mismatch

---@type integer
local count = wraith.paths.stdpath("data") -- expect: assign-type-mismatch
print(count)

local manager = require("wraith.manager")
manager.setup(false) -- expect: param-type-mismatch
manager.plan("fetch") -- expect: param-type-mismatch
wraith.buffers.get(1) -- expect: param-type-mismatch
wraith.windows.split("diagonal") -- expect: param-type-mismatch
wraith.fs.read(false) -- expect: param-type-mismatch
wraith.process.start(false, function() end) -- expect: param-type-mismatch
wraith.http.request(false, function() end) -- expect: param-type-mismatch
wraith.jobs.output(false, function() end) -- expect: param-type-mismatch
wraith.invocation.fail("id", false) -- expect: param-type-mismatch
wraith.providers.register("review", false) -- expect: param-type-mismatch
wraith.tools.register("review", false) -- expect: param-type-mismatch

wraith.hooks.add("before_request", function(ctx)
  ---@type integer
  local model = ctx.request.model -- expect: assign-type-mismatch
  print(model)
  return { request = ctx.request }
end)

---@type Wraith.PackageOperation
local operation = { id = "invalid", phase = "publishing", state = "invented", restart_required = {} } -- expect: assign-type-mismatch
print(operation)

wraith.modules.provider(false) -- expect: param-type-mismatch
wraith.modules.remove_provider(1) -- expect: param-type-mismatch
wraith.plugins.unload_many("plugins.review") -- expect: param-type-mismatch
wraith.plugins.unload_many() -- expect: missing-parameter
wraith.plugins.unload_many({ false }) -- expect: assign-type-mismatch

---@type Wraith.ModuleCheckPhase
local phase = "resolve" -- expect: assign-type-mismatch
---@type Wraith.ModuleResolution
local descriptor = { plugin = false, activate = function() end, check = function() end } -- expect: assign-type-mismatch
---@type Wraith.ModuleResolver
local invalid_resolver = function() return false end -- expect: return-type-mismatch
---@type Wraith.ModuleResolution
local invalid_activation = { plugin = "review", activate = false, check = function() end } -- expect: assign-type-mismatch
---@type Wraith.ModuleCheck
local invalid_check = function(name, current, proof, executed)
  ---@type string
  local text = executed -- expect: assign-type-mismatch
  print(name, current, proof, text)
end
invalid_check("review", "before", nil, "yes") -- expect: param-type-mismatch
print(phase, descriptor, invalid_resolver, invalid_activation, invalid_check)
