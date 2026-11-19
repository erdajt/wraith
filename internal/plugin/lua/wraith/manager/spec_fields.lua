local M = {}
local allowed = {name=true, repo=true, url=true, dir=true, main=true, dependencies=true, enabled=true,
  priority=true, lazy=true, cmd=true, event=true, keys=true, init=true, opts=true, config=true,
  branch=true, tag=true, commit=true, pin=true}

function M.array(value, label, limit)
  assert(type(value) == "table", label .. " must be an array")
  local count = 0
  for key in pairs(value) do
    assert(type(key) == "number" and key % 1 == 0 and key >= 1 and key <= #value, label .. " must be consecutive")
    count = count + 1
  end
  assert(count == #value and count <= limit, label .. " exceeds its limit")
  return value
end

function M.copy(value)
  local active, count = {}, 0
  local function visit(item, depth)
    if type(item) ~= "table" then return item end
    assert(depth <= 16 and not active[item], "package options contain a cycle or exceed depth 16")
    active[item] = true
    local result = {}
    for key, child in pairs(item) do
      count = count + 1
      assert(count <= 4096 and (type(key) == "string" or type(key) == "number"), "package options exceed their limit or contain invalid keys")
      result[key] = visit(child, depth + 1)
    end
    active[item] = nil
    return result
  end
  return visit(value, 1)
end

local function strings(value, label)
  if value == nil then return {} end
  if type(value) == "string" then value = {value} end
  local result, seen = {}, {}
  for _, item in ipairs(M.array(value, label, 64)) do
    assert(type(item) == "string" and #item > 0 and #item <= 128 and not item:find("%c"), "invalid " .. label)
    assert(not seen[item], "duplicate " .. label .. ": " .. item)
    seen[item], result[#result + 1] = true, item
  end
  return result
end

function M.normalize(value, descriptor)
  if type(value) == "string" then value = {repo=value} end
  assert(type(value) == "table", "package spec must be a table or repository")
  for key in pairs(value) do
    assert(key == 1 or allowed[key], "unknown package field: " .. tostring(key))
  end
  assert(value[1] == nil or value.repo == nil, "choose one repository field")
  local repo = value.repo
  if repo == nil then repo = value[1] end
  local sources = (repo ~= nil and 1 or 0) + (value.url ~= nil and 1 or 0) + (value.dir ~= nil and 1 or 0)
  assert(sources == 1, "choose exactly one repo, url or dir")
  local location, source = value.dir or value.url or repo
  assert(type(location) == "string" and #location > 0, "package source must be a nonempty string")
  if value.dir then
    if location:sub(1, 1) ~= "/" then location = wraith.paths.stdpath("config") .. "/" .. location end
    local parts = {}
    for part in location:gmatch("[^/]+") do
      if part == ".." then table.remove(parts) elseif part ~= "." then parts[#parts + 1] = part end
    end
    location = "/" .. table.concat(parts, "/")
    source = {kind="local", dir=location}
  elseif repo then
    assert(repo:match("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$"), "repo must be owner/repository; use url for an explicit remote")
    source = {kind="git", url="https://github.com/" .. repo .. (repo:sub(-4) == ".git" and "" or ".git")}
  else
    source = {kind="git", url=location}
  end

  local basename = location:gsub("/+$", ""):match("([^/:]+)$") or ""
  basename = basename:gsub("%.git$", "")
  local selected, count = {kind="head"}, 0
  for _, kind in ipairs({"branch", "tag", "commit"}) do
    if value[kind] ~= nil then selected, count = {kind=kind, value=value[kind]}, count + 1 end
  end
  assert(count <= 1, "choose one branch, tag or commit")
  local result = descriptor({name=value.name == nil and basename:lower() or value.name, source=source, selector=selected, pin=value.pin})
  result.main = value.main == nil and basename or value.main
  assert(type(result.main) == "string" and #result.main <= 128 and result.main:match("^[A-Za-z0-9_-]+[A-Za-z0-9_.-]*$") and not result.main:find("..", 1, true), "invalid package main module")
  assert(result.main ~= "wraith.manager" and result.main:sub(1, 15) ~= "wraith.manager.", "manager module namespace is reserved")
  assert(value.enabled == nil or type(value.enabled) == "boolean" or type(value.enabled) == "function", "enabled must be boolean or function")
  result.enabled = value.enabled ~= false
  if type(value.enabled) == "function" then
    result.enabled = value.enabled(M.copy(value))
    assert(type(result.enabled) == "boolean", "enabled must return boolean")
  end
  result.priority = value.priority == nil and 0 or value.priority
  assert(type(result.priority) == "number" and result.priority % 1 == 0 and math.abs(result.priority) <= 1000000, "priority must be an integer within 1000000")
  assert(value.lazy == nil or type(value.lazy) == "boolean", "lazy must be boolean")
  assert(value.init == nil or type(value.init) == "function", "init must be a function")
  assert(value.opts == nil or type(value.opts) == "table" or type(value.opts) == "function", "opts must be a table or function")
  assert(value.config == nil or type(value.config) == "boolean" or type(value.config) == "function", "config must be boolean or function")
  result.init, result.opts, result.config = value.init, M.copy(value.opts), value.config
  result.cmd, result.event, result.keys = strings(value.cmd, "command trigger"), strings(value.event, "event trigger"), {}
  for _, command in ipairs(result.cmd) do
    assert(#command <= 80 and command:match("^[a-z][a-z0-9.%-]*!?$") and not command:find("..", 1, true) and not command:find("%.!*$"), "invalid command trigger: " .. command)
    for segment in command:gsub("!$", ""):gmatch("[^.]+") do
      assert(segment:match("^[a-z0-9][a-z0-9%-]*$"), "invalid command trigger segment: " .. command)
    end
  end
  for _, event in ipairs(result.event) do wraith.events.listeners(event) end
  for _, key in ipairs(M.array(value.keys == nil and {} or value.keys, "key triggers", 64)) do
    assert(type(key) == "table", "key trigger requires explicit keys and action")
    for field in pairs(key) do
      assert(field == 1 or field == 2 or field == "keys" or field == "action" or field == "mode" or field == "desc" or field == "description", "unknown key trigger field")
    end
    assert(not (key[1] and key.keys) and not (key[2] and key.action) and not (key.desc and key.description), "duplicate key trigger fields")
    local mapping = {keys=key.keys == nil and key[1] or key.keys, action=key.action == nil and key[2] or key.action, mode=key.mode == nil and "chat.normal" or key.mode, description=key.description == nil and (key.desc == nil and "load " .. result.name or key.desc) or key.description}
    assert(type(mapping.keys) == "string" and #mapping.keys > 0 and #mapping.keys <= 256 and not mapping.keys:find("%c"), "invalid key trigger keys")
    assert(type(mapping.action) == "function" or type(mapping.action) == "string" and #mapping.action > 0 and #mapping.action <= 256, "key trigger requires an explicit action or function")
    assert(type(mapping.mode) == "string" and type(mapping.description) == "string", "invalid key trigger mode or description")
    wraith.keymap.validate(mapping.mode, mapping.keys)
    result.keys[#result.keys + 1] = mapping
  end
  result.lazy = value.lazy
  if result.lazy == nil then result.lazy = #result.cmd + #result.event + #result.keys > 0 end
  return result
end

return M
