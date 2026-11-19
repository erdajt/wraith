local process = require("wraith.manager.store_process")
local M = {}
local allowed = { root = true, lockfile = true, timeout_ms = true, lock_timeout_ms = true, git_env = true }

local function path(value, label)
  assert(type(value) == "string" and #value <= 4096 and value:sub(1, 1) == "/" and not value:find("%c") and not value:find("[;?]"), label .. " must be an absolute path without controls or module-path delimiters")
  local parts = {}
  for part in value:gmatch("[^/]+") do
    if part == ".." then table.remove(parts) elseif part ~= "." then parts[#parts + 1] = part end
  end
  return "/" .. table.concat(parts, "/")
end

local function timeout(value, fallback, maximum, label)
  if value == nil then value = fallback end
  assert(type(value) == "number" and value % 1 == 0 and value >= 1 and value <= maximum, label .. " is outside its integer range")
  return value
end

function M.normalize(options)
  options = options == nil and {} or options
  assert(type(options) == "table", "storage options must be a table")
  for key in pairs(options) do assert(allowed[key], "unknown storage option: " .. tostring(key)) end
  local result = {
    root = path(options.root == nil and wraith.paths.stdpath("data") .. "/packages" or options.root, "package root"),
    lockfile = path(options.lockfile == nil and wraith.paths.stdpath("config") .. "/wraith-lock.json" or options.lockfile, "package lock"),
    timeout_ms = timeout(options.timeout_ms, 300000, 86400000, "package timeout"),
    lock_timeout_ms = timeout(options.lock_timeout_ms, 5000, 300000, "lock timeout"),
    git_env = {},
  }
  assert(result.lockfile ~= "/", "package lock requires a filename")
  assert(options.git_env == nil or type(options.git_env) == "table", "git_env must be a table")
  process.environment(options.git_env)
  for key, value in pairs(options.git_env or {}) do result.git_env[key] = value end
  return result
end

return M
