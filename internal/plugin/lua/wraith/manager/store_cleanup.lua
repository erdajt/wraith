local files = require("wraith.manager.store_files")
local flow = require("wraith.manager.store_flow")
local M = {}

function M.start(workspace, restore, completed)
  local control = flow.new(completed)
  local paths = { { workspace.stage, true }, { workspace.baseline, false } }
  if restore then paths[#paths + 1] = { workspace.candidate, false } end
  local errors, remove = {}, nil
  remove = function(index)
    local entry = paths[index]
    if not entry then
      control.finish({ state = #errors == 0 and "completed" or "failed", error = #errors > 0 and table.concat(errors, "; ") or nil })
      return
    end
    control.begin(function(callback)
      return files.start("remove", { entry[1], { recursive = entry[2] } }, callback)
    end, function(result)
      if result.state ~= "completed" then errors[#errors + 1] = tostring(result.error):sub(1, 1024) end
      remove(index + 1)
    end)
  end
  remove(1)
  return control
end

return M
