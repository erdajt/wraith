local process = require("wraith.manager.store_process")
local lock = require("wraith.manager.store_lock")
local M = {}
local transaction = [[
set -eu
lock=$1
baseline=$2
candidate=$3
absent=$4
if [ -L "$lock" ] || { [ -e "$lock" ] && [ ! -f "$lock" ]; }; then exit 73; fi
if [ -L "$candidate" ] || [ ! -f "$candidate" ]; then exit 73; fi
if [ "$absent" = yes ]; then
  if [ -e "$lock" ]; then exit 75; fi
else
  if [ -L "$baseline" ] || [ ! -f "$baseline" ]; then exit 73; fi
  if ! cmp -s -- "$baseline" "$lock"; then exit 75; fi
fi
mv -T -- "$candidate" "$lock"
]]

function M.commit(request, options, completed)
  local parent = assert(request.lockfile:match("^(.*)/[^/]+$"), "lockfile must be absolute")
  if parent == "" then parent = "/" end
  assert(request.lockfile:sub(1, 1) == "/", "lockfile must be absolute")
  local seen = { [request.lockfile] = true }
  for _, path in ipairs({ request.guard, request.baseline, request.candidate }) do
    local directory = type(path) == "string" and path:match("^(.*)/[^/]+$")
    if directory == "" then directory = "/" end
    assert(directory == parent and not seen[path], "publication files must be distinct siblings")
    seen[path] = true
  end
  assert(wraith.fs.stat(request.guard).type == "file", "package guard must be a regular file")
  local argv = { "flock", "--exclusive", "--timeout", tostring((options.lock_timeout_ms or 5000) / 1000),
    "--conflict-exit-code", "74", "--no-fork", request.guard, "/bin/sh", "-c", transaction,
    "wraith-publish", request.lockfile, request.baseline, request.candidate, request.absent and "yes" or "no" }
  return process.start(argv, options, function(result)
    if result.state == "completed" and result.exit_code == 0 then
      completed({ state = "installed", generation = request.generation })
      return
    end
    if result.exit_code == 75 then
      completed({ state = "conflict", error = "package lock changed; create a fresh plan" })
      return
    end
    if result.exit_code == 74 then
      completed({ state = "failed", error = "package lock acquisition timed out" })
      return
    end
    if result.exit_code == 73 then
      completed({ state = "failed", error = "publication requires regular files, not directories or symlinks" })
      return
    end

    local ok, current, raw = pcall(lock.read, request.lockfile)
    if ok and current.generation == request.generation then
      completed({ state = "installed", generation = request.generation, message = "publication confirmed after process interruption" })
      return
    end
    local unchanged = ok and request.absent and raw == nil
    if ok and not request.absent then
      local read, baseline = pcall(wraith.fs.read, request.baseline, { max_bytes = 1048576 })
      unchanged = read and not baseline.truncated and raw == baseline.data
    end
    completed({ state = unchanged and result.state or "uncertain", error = unchanged and result.error or "publication outcome is uncertain; inspect the current lock before retrying" })
  end)
end

return M
