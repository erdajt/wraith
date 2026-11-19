local M = {}
local runtime = "wraith.manager.runtime"

function M.setup(specs, options)
  options = options == nil and {} or options
  assert(type(options) == "table", "manager options must be a table")
  for key in pairs(options) do assert(key == "ui" or key == "storage", "unknown manager option: " .. tostring(key)) end
  for _, name in ipairs(wraith.plugins.list()) do assert(name ~= runtime, "package manager is already configured") end
  local graph = require("wraith.manager.spec").normalize(specs)
  assert(#graph.ordered * 2 + #wraith.plugins.list() + 1 <= 128, "managed packages exceed available lifecycle capacity")
  wraith.plugins.load(runtime, {}, {dependency=false})
  local ok, err = pcall(wraith.plugins.call, runtime, "configure", graph, require("wraith.manager.spec_fields").copy(options))
  if not ok then
    local unloaded, failure = pcall(wraith.plugins.unload, runtime)
    if not unloaded then error(tostring(err) .. "; manager cleanup: " .. tostring(failure)) end
    error(err)
  end
  return M
end

for _, method in ipairs({"status", "load", "unload", "unload_all", "plan", "execute", "install", "update", "restore", "cancel", "recover", "open"}) do
  M[method] = function(...) return wraith.plugins.call(runtime, method, ...) end
end

return M
