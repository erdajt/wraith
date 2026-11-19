local git = require("wraith.manager.store_git_command")
local files = require("wraith.manager.store_files")
local M = {}

function M.new(options, completed)
  local control, current, finished, cancelled = {}, nil, false, false

  function control.finish(result)
    if finished then return end
    finished = true
    completed(result)
  end

  local function start(factory, next, process)
    if cancelled then control.finish({ state = "cancelled", error = "package installation cancelled" }); return end
    current = factory(function(result)
      current = nil
      if cancelled or result.state == "cancelled" then
        control.finish({ state = "cancelled", error = result.error or "package installation cancelled" })
      elseif result.state ~= "completed" or process and result.exit_code ~= 0 or result.truncated then
        control.finish({ state = "failed", error = result.truncated and "git output exceeded its capture limit" or result.error or "package operation failed", stderr = result.stderr })
      else
        local ok, err = pcall(next, process and result.stdout or result.result)
        if not ok then
          if finished then error(err) end
          control.finish({ state = "failed", error = tostring(err) })
        end
      end
    end)
  end

  function control.run(arguments, directory, next)
    start(function(callback) return git.run(arguments, directory, options, callback) end, next, true)
  end

  function control.file(method, arguments, next)
    start(function(callback) return files.start(method, arguments, callback) end, next, false)
  end

  function control.cancel()
    cancelled = true
    if current then current.cancel() end
  end

  return control
end

return M
