local M = {}

function M.inspect(options)
  local parent = options.lockfile:match("^(.*)/[^/]+$")
  if parent == "" then parent = "/" end
  local report = {
    root = options.root, lock_directory = parent,
    staging = wraith.json.array(), publication = wraith.json.array(),
    errors = wraith.json.array(), truncated = { staging = false, publication = false },
  }
  for _, source in ipairs({ { "staging", options.root .. "/staging" }, { "publication", parent } }) do
    local kind, directory = source[1], source[2]
    local ok, result = pcall(function()
      if kind == "staging" then
        assert(wraith.fs.stat(options.root).type == "directory", "managed package root must be a directory")
        assert(wraith.fs.stat(directory).type == "directory", "managed staging path must be a directory")
      end
      return wraith.fs.list(directory, { limit = 1024 })
    end)
    if not ok then
      report.errors[#report.errors + 1] = { path = directory, error = tostring(result) }
    else
      report.truncated[kind] = result.truncated
      for _, entry in ipairs(result.entries) do
        if kind == "staging" or entry.name:match("^%.wraith%-candidate%.") or entry.name:match("^%.wraith%-baseline%.") then
          report[kind][#report[kind] + 1] = entry.name
        end
      end
    end
  end
  return report
end

return M
