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
