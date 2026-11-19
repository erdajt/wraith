local M = {}

local function copy(value)
  return wraith.json.decode(wraith.json.encode(value))
end

local function equal(left, right)
  return wraith.json.encode(left) == wraith.json.encode(right)
end

function M.create(specs, lock, action, names)
  assert(action == "install" or action == "update" or action == "restore", "unknown package operation")
  specs, lock = copy(specs), copy(lock)
  local indexed, selected, visiting, ordered = {}, {}, {}, {}
  for _, spec in ipairs(specs) do
    assert(not indexed[spec.name], "duplicate package: " .. spec.name)
    indexed[spec.name] = spec
  end

  local function select(name)
    assert(type(name) == "string" and indexed[name], "unknown package: " .. tostring(name))
    assert(not visiting[name], "package dependency cycle: " .. name)
    if selected[name] then return end
    visiting[name] = true
    for _, dependency in ipairs(indexed[name].dependencies or {}) do select(dependency) end
    visiting[name], selected[name] = nil, true
    ordered[#ordered + 1] = indexed[name]
  end

  if names == nil then
    for _, spec in ipairs(specs) do select(spec.name) end
  else
    assert(type(names) == "table" and #names > 0, "choose at least one package")
    local count = 0
    for key in pairs(names) do
      assert(type(key) == "number" and key >= 1 and key <= #names and key % 1 == 0, "package names must be an array")
      count = count + 1
    end
    assert(count == #names, "package names must be consecutive")
    for _, name in ipairs(names) do select(name) end
  end

  local packages = {}
  for _, spec in ipairs(ordered) do
    local previous = lock.packages[spec.name]
    local item = { name = spec.name, source = spec.source, selector = spec.selector, pin = spec.pin }
    if spec.source.kind == "local" then
      assert(action ~= "restore" or not previous, "local package has a git lock; reconcile with install: " .. spec.name)
      item.state, item.path = "local", spec.source.dir
    else
      assert(not previous or equal(previous.source, spec.source), "package source changed: " .. spec.name)
      if action == "restore" then
        assert(previous, "package has no locked revision: " .. spec.name)
        item.commit = previous.commit
      elseif action == "install" and previous then
        assert(equal(previous.selector, spec.selector), "package selector changed; update " .. spec.name)
        item.commit = previous.commit
      elseif spec.selector.kind == "commit" then
        item.commit = spec.selector.value
      elseif spec.pin and previous then
        item.commit = previous.commit
      end
      item.previous = previous and previous.commit or nil
      item.state = item.commit and "locked" or "resolve"
    end
    packages[#packages + 1] = item
  end

  local removed = {}
  if action ~= "restore" then
    for name in pairs(lock.packages) do
      if not indexed[name] or indexed[name].source.kind == "local" then removed[#removed + 1] = name end
    end
    table.sort(removed)
  end
  return { action = action, packages = packages, baseline = lock, removed = removed }
end

return M
