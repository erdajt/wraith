local M = {}

local function fields(value, allowed, label)
  assert(type(value) == "table", label .. " must be a table")
  for key in pairs(value) do
    assert(type(key) == "string" and allowed[key], "unknown " .. label .. " field: " .. tostring(key))
  end
end

local function name(value)
  assert(type(value) == "string" and #value <= 64 and value:match("^[a-z0-9][a-z0-9._-]*$"), "invalid package name")
  return value
end

local function text(value, label)
  assert(type(value) == "string" and #value > 0 and #value <= 4096 and not value:find("%c"), "invalid " .. label)
  return value
end

local function source(value)
  fields(value, {kind=true, url=true, dir=true}, "source")
  if value.kind == "local" then
    assert(value.url == nil, "local source cannot contain a url")
    local path = text(value.dir, "local package directory")
    assert(path:sub(1, 1) == "/" and not path:find("[;?]"), "local package directory must be an explicit absolute module root")
    return {kind="local", dir=path}
  end
  assert(value.kind == "git" and value.dir == nil, "source must be git or local")
  local url = text(value.url, "git source")
  assert(url:sub(1, 1) ~= "-" and not url:find("[?#]"), "git source cannot contain options, query credentials or fragments")
  if url:sub(1, 1) ~= "/" then
    assert(not url:find("%s"), "remote git source cannot contain whitespace")
    local protocol, authority = url:match("^([a-z]+)://([^/]+)")
    if protocol then
      assert(protocol == "https" or protocol == "ssh" or protocol == "file", "unsupported git source protocol")
      if protocol == "ssh" then
        local user = authority:match("^(.-)@")
        assert(not user or user:match("^[A-Za-z0-9_.-]+$"), "git source cannot contain credentials")
        local host = authority:match("@(.+)$") or authority
        assert(host:sub(1, 1) ~= "-", "ssh host cannot begin with an option")
      else
        assert(not authority:find("@", 1, true), "git source cannot contain credentials")
      end
    elseif url:sub(1, 8) == "file:///" then
      assert(#url > 8, "git file source requires a path")
    else
      local host, path = url:match("^([^:]+):(.+)$")
      local user = host and host:match("^(.-)@")
      host = host and (host:match("@(.+)$") or host)
      assert(host and host:match("^[A-Za-z0-9_.-]+$") and host:sub(1, 1) ~= "-", "git source requires a valid ssh host, url or absolute path")
      assert(not user or user:match("^[A-Za-z0-9_.-]+$"), "git source cannot contain credentials")
      assert(path:sub(1, 1) ~= "-" and path:sub(1, 1) ~= ":", "ssh repository path cannot begin with an option or protocol delimiter")
    end
  end
  return {kind="git", url=url}
end

local function selector(value)
  fields(value, {kind=true, value=true}, "selector")
  local kind = value.kind
  if kind == "head" then
    assert(value.value == nil, "head selector cannot contain a value")
    return {kind="head"}
  end
  assert(kind == "branch" or kind == "tag" or kind == "commit", "unknown package selector")
  local ref = text(value.value, "package ref")
  if kind == "commit" then
    assert((#ref == 40 or #ref == 64) and ref:match("^[a-fA-F0-9]+$"), "commit must be a full git object id")
    return {kind=kind, value=ref:lower()}
  end
  assert(#ref <= 255, "package ref exceeds 255 bytes")
  assert(ref ~= "@" and ref:sub(1, 1) ~= "-" and not ref:find("[%s~^:?*%[\\]") and not ref:find("..", 1, true) and not ref:find("@{", 1, true), "invalid package ref")
  assert(ref:sub(1, 1) ~= "/" and ref:sub(-1) ~= "/" and not ref:find("//", 1, true), "invalid package ref path")
  for part in ref:gmatch("[^/]+") do
    assert(part:sub(1, 1) ~= "." and part:sub(-1) ~= "." and part:sub(-5) ~= ".lock", "invalid package ref component")
  end
  return {kind=kind, value=ref}
end

function M.descriptor(value)
  fields(value, {name=true, source=true, selector=true, pin=true, dependencies=true}, "package descriptor")
  assert(value.pin == nil or type(value.pin) == "boolean", "pin must be boolean")
  local result = {name=name(value.name), source=source(value.source), selector=selector(value.selector or {kind="head"}), pin=value.pin or false, dependencies={}}
  if result.source.kind == "local" then
    assert(result.selector.kind == "head" and not result.pin, "local packages cannot select or pin git revisions")
  end
  local dependencies, seen = value.dependencies or {}, {}
  assert(type(dependencies) == "table", "dependencies must be an array")
  local count = 0
  for key in pairs(dependencies) do
    assert(type(key) == "number" and key >= 1 and key <= #dependencies and key % 1 == 0, "dependencies must be consecutive")
    count = count + 1
  end
  assert(count == #dependencies and count <= 63, "dependencies exceed the package limit")
  for _, dependency in ipairs(dependencies) do
    dependency = name(dependency)
    assert(not seen[dependency] and dependency ~= result.name, "duplicate or self dependency: " .. dependency)
    seen[dependency] = true
    result.dependencies[#result.dependencies + 1] = dependency
  end
  return result
end

return M
