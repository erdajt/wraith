local wait = require("wraith.manager.store_wait")
local M = {}
local methods = { read = true, write = true, stat = true, list = true, mkdir = true, remove = true, rename = true }

function M.start(method, arguments, completed)
  assert(methods[method], "unsupported package filesystem operation")
  assert(type(completed) == "function", "package filesystem callback is required")
  local control, values, finished = {}, {}, false
  for _, value in ipairs(arguments) do values[#values + 1] = value end
  values[#values + 1] = function(ctx)
    if not ctx.resource.final then return end
    wait.finish(control.id, function(status)
      finished = true
      completed({ state = status.state, error = status.error, result = status.result })
    end)
  end
  control.id = wraith.fs[method .. "_async"](unpack(values))

  function control.cancel()
    if not finished then wraith.resource.cancel(control.id) end
  end

  return control
end

return M
