local M, selected, count = {}, {}, 0

-- module caches and runtime paths survive package unload, so source identities do too.
function M.check(spec, path, commit)
  local prior = selected[spec.name]
  if prior then
    local source = spec.source
    assert(prior.main == spec.main and prior.path == path and prior.commit == commit
      and prior.kind == source.kind and prior.url == source.url and prior.dir == source.dir,
      "package source changed in this runtime; use /reload: " .. spec.name)
  end
  for name, identity in pairs(selected) do
    assert(name == spec.name or identity.main ~= spec.main,
      "package main was already selected by " .. name .. "; use /reload: " .. spec.main)
  end
end

function M.reserve(spec, path, commit)
  M.check(spec, path, commit)
  if selected[spec.name] then return end
  assert(count < 63, "package source identities exceed runtime capacity; use /reload")
  local source = spec.source
  selected[spec.name] = {main=spec.main, path=path, commit=commit, kind=source.kind, url=source.url, dir=source.dir}
  count = count + 1
end

return M
