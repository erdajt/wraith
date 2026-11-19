local flow = require("wraith.manager.store_flow")
local files = require("wraith.manager.store_files")
local process = require("wraith.manager.store_process")
local M = {}
local tools = 'for tool in git flock cmp mv mktemp; do command -v "$tool" >/dev/null || exit 127; done'

function M.prepare(options, baseline, completed)
  local control = flow.new(completed)
  local parent = options.lockfile:match("^(.*)/[^/]+$")
  if parent == "" then parent = "/" end
  local result = { lockfile = options.lockfile, guard = parent .. "/.wraith-packages.lock", absent = baseline == nil }

  local function file(method, arguments, next)
    control.begin(function(callback) return files.start(method, arguments, callback) end, function(value)
      assert(value.state == "completed", value.error)
      next(value.result)
    end)
  end

  local function temporary(prefix, directory, next)
    local argv = { "mktemp" }
    if directory then argv[#argv + 1] = "--directory" end
    argv[#argv + 1], argv[#argv + 2] = "--", prefix .. "XXXXXXXX"
    control.begin(function(callback) return process.start(argv, options, callback) end, function(value)
      assert(value.state == "completed" and not value.truncated, value.error or "invalid temporary path output")
      local path = value.stdout:match("^([^\r\n]+)\n$")
      assert(path and path:sub(1, #prefix) == prefix and path:sub(#prefix + 1):match("^[A-Za-z0-9]+$"), "mktemp returned an unexpected path")
      next(path)
    end)
  end

  local function guard()
    control.begin(function(callback) return files.start("write", { result.guard, "", { overwrite = false } }, callback) end, function()
      file("stat", { result.guard }, function(value)
        assert(value.type == "file", "package guard must be a regular file")
        temporary(parent .. "/.wraith-baseline.", false, function(path)
          result.baseline = path
          file("write", { path, baseline or "", { overwrite = true } }, function()
            temporary(parent .. "/.wraith-candidate.", false, function(candidate)
              result.candidate = candidate
              control.finish({ state = "completed", workspace = result })
            end)
          end)
        end)
      end)
    end)
  end

  local function staging()
    local path = options.root .. "/staging"
    control.begin(function(callback) return files.start("mkdir", { path, { mode = 448 } }, callback) end, function()
      file("stat", { path }, function(value)
        assert(value.type == "directory", "staging root must be a directory, not a symlink")
        temporary(path .. "/txn.", true, function(stage)
          result.stage, result.generation = stage, stage:match("([^/]+)$")
          guard()
        end)
      end)
    end)
  end

  control.begin(function(callback) return process.start({ "/bin/sh", "-c", tools }, options, callback) end, function(value)
    assert(value.state == "completed", "package installation requires git, flock, cmp, mv and mktemp: " .. tostring(value.error))
    file("mkdir", { options.root, { parents = true, mode = 448 } }, function()
      file("stat", { options.root }, function(root)
        assert(root.type == "directory", "package root must be a directory, not a symlink")
        staging()
      end)
    end)
  end)
  return control
end

return M
