local descriptor = require("wraith.manager.spec").descriptor
local M = {}

local function fields(value, allowed, label)
  assert(type(value) == "table", label .. " must be an object")
  for key in pairs(value) do
    assert(type(key) == "string" and allowed[key], "unknown " .. label .. " field: " .. tostring(key))
  end
end

function M.empty()
  return { version = 1, generation = "initial", packages = wraith.json.object() }
end

function M.decode(text)
  local value = wraith.json.decode(text)
  fields(value, { version = true, generation = true, packages = true }, "package lock")
  assert(value.version == 1, "unsupported package lock version")
  assert(type(value.generation) == "string" and #value.generation <= 128 and value.generation:match("^[A-Za-z0-9_.-]+$"), "invalid package lock generation")
  assert(type(value.packages) == "table" and wraith.json.encode(value.packages):sub(1, 1) == "{", "locked packages must be an object")

  local result, count = M.empty(), 0
  result.generation = value.generation
  for name, entry in pairs(value.packages) do
    count = count + 1
    assert(count <= 63, "package lock exceeds 63 entries")
    fields(entry, { source = true, selector = true, pin = true, commit = true }, "locked package")
    assert(type(entry.source) == "table" and type(entry.selector) == "table" and type(entry.pin) == "boolean", "locked source, selector and pin are required")
    local spec = descriptor({ name = name, source = entry.source, selector = entry.selector, pin = entry.pin })
    assert(spec.source.kind == "git", "local packages do not belong in the lock")
    local commit = entry.commit
    assert(type(commit) == "string" and (#commit == 40 or #commit == 64) and commit:match("^[a-fA-F0-9]+$"), "invalid locked commit: " .. name)
    commit = commit:lower()
    assert(spec.selector.kind ~= "commit" or spec.selector.value == commit, "locked commit does not match its selector: " .. name)
    result.packages[name] = { source = spec.source, selector = spec.selector, pin = spec.pin, commit = commit }
  end
  return result
end

function M.read(path)
  assert(type(path) == "string" and path:sub(1, 1) == "/", "package lock path must be absolute")
  local ok, stat = pcall(wraith.fs.stat, path)
  if not ok then
    local parent, name = path:match("^(.*)/([^/]+)$")
    assert(parent, "package lock path must be absolute")
    if parent == "" then parent = "/" end
    local offset = 0
    for _ = 1, 4 do
      local listed, page = pcall(wraith.fs.list, parent, { offset = offset, limit = 1024 })
      assert(listed, "package lock parent must exist and be readable: " .. parent .. "; " .. tostring(page))
      for _, entry in ipairs(page.entries) do
        assert(entry.name ~= name, "cannot inspect package lock: " .. tostring(stat))
      end
      if not page.truncated then return M.empty(), nil end
      offset = page.next_offset
    end
    error("cannot establish missing lock in a directory exceeding 4096 entries")
  end
  assert(stat.type == "file", "package lock must be a regular file")
  local file = wraith.fs.read(path, { max_bytes = 1048576 })
  assert(not file.truncated, "package lock exceeds 1 mib")
  return M.decode(file.data), file.data
end

return M
