local lock = require("wraith.manager.store_lock")
local M = {}

function M.read(specs, options)
  local current, raw = lock.read(options.lockfile)
  local rows, directories = wraith.json.array(), {}
  local function directory(path)
    if directories[path] == nil then
      local ok, stat = pcall(wraith.fs.stat, path)
      directories[path] = ok and stat.type == "directory" and true or ok and "managed package path is not a directory" or tostring(stat)
    end
    return directories[path]
  end
  for _, spec in ipairs(specs) do
    local row = { name = spec.name, source = spec.source, selector = spec.selector, pin = spec.pin }
    local entry = current.packages[spec.name]
    if spec.source.kind == "local" then
      row.state = "local"
      local ok, stat = pcall(wraith.fs.stat, spec.source.dir, { follow = true })
      if ok and stat.type == "directory" then row.path = spec.source.dir else row.error = "local package directory is unavailable" end
    elseif entry and wraith.json.encode(entry.source) ~= wraith.json.encode(spec.source) then
      row.state, row.error = "source-conflict", "package source changed; use a distinct name or reconcile the lock"
    elseif entry then
      row.commit, row.state = entry.commit, "missing"
      local path = options.root .. "/objects/" .. spec.name .. "/" .. entry.commit
      local usable = true
      for _, parent in ipairs({ options.root, options.root .. "/objects", options.root .. "/objects/" .. spec.name, path }) do
        local result = directory(parent)
        if result ~= true then usable, row.error = false, result; break end
      end
      if usable then row.path, row.state = path, "installed" end
    else
      row.state = "missing"
    end
    rows[#rows + 1] = row
  end
  return wraith.json.decode(wraith.json.encode(rows)), current, raw
end

return M
