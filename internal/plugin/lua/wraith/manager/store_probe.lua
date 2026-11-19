local flow = require("wraith.manager.store_flow")
local files = require("wraith.manager.store_files")
local M = {}

function M.object(root, name, commit, completed)
  local control = flow.new(completed)
  local paths = { root .. "/objects", root .. "/objects/" .. name, root .. "/objects/" .. name .. "/" .. commit }
  local inspect
  inspect = function(index)
    control.begin(function(callback) return files.start("stat", { paths[index] }, callback) end, function(result)
      if result.state == "completed" then
        assert(result.result.type == "directory", "managed object path must be a directory, not a symlink")
        if index == #paths then control.finish({ state = "completed", exists = true, path = paths[index] }) else inspect(index + 1) end
        return
      end
      local parent, filename = paths[index]:match("^(.*)/([^/]+)$")
      local original, scan = result.error, nil
      scan = function(offset, page)
        control.begin(function(callback) return files.start("list", { parent, { offset = offset, limit = 1024 } }, callback) end, function(list)
          assert(list.state == "completed", list.error)
          for _, entry in ipairs(list.result.entries) do assert(entry.name ~= filename, original) end
          if not list.result.truncated then
            control.finish({ state = "completed", exists = false, path = paths[#paths] })
          else
            assert(page < 4, "cannot establish missing managed object in a directory exceeding 4096 entries")
            scan(list.result.next_offset, page + 1)
          end
        end)
      end
      scan(0, 1)
    end)
  end
  inspect(1)
  return control
end

return M
