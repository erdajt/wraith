local files = require("wraith.manager.store_files")
local verify = require("wraith.manager.store_verify")
local M = {}

function M.install(item, stage, root, options, completed)
  local control, current, finished, cancelled = {}, nil, false, false
  local parents = { root, root .. "/objects", root .. "/objects/" .. item.name }
  local destination = parents[3] .. "/" .. item.commit

  local function finish(result)
    if finished then return end
    finished = true
    completed(result)
  end

  local function begin(factory, next)
    if cancelled then finish({ state = "cancelled", error = "package promotion cancelled" }); return end
    current = factory(function(result)
      current = nil
      if cancelled or result.state == "cancelled" then
        finish({ state = "cancelled", error = "package promotion cancelled" })
        return
      end
      local ok, err = pcall(next, result)
      if not ok then
        if finished then error(err) end
        finish({ state = "failed", error = tostring(err) })
      end
    end)
  end

  local function promote()
    begin(function(callback) return files.start("rename", { stage, destination, { overwrite = false } }, callback) end, function(result)
      if result.state == "completed" then
        finish({ state = "completed", path = destination, commit = item.commit, reused = false })
        return
      end
      begin(function(callback) return verify.check(item, destination, options, callback) end, function(existing)
        if existing.state ~= "completed" then
          finish({ state = "failed", error = tostring(result.error) .. "; destination not reusable: " .. tostring(existing.error) })
          return
        end
        existing.reused = true
        finish(existing)
      end)
    end)
  end

  local check
  check = function(index)
    local function inspect()
      begin(function(callback) return files.start("stat", { parents[index] }, callback) end, function(result)
        assert(result.state == "completed", result.error)
        assert(result.result.type == "directory", "managed object parent must be a directory, not a symlink")
        if index == #parents then promote() else check(index + 1) end
      end)
    end
    if index == 1 then inspect() else
      begin(function(callback) return files.start("mkdir", { parents[index], { mode = 448 } }, callback) end, inspect)
    end
  end
  check(1)

  function control.cancel()
    cancelled = true
    if current then current.cancel() end
  end

  return control
end

return M
