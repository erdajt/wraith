local flow = require("wraith.manager.store_flow")
local files = require("wraith.manager.store_files")
local workspace = require("wraith.manager.store_workspace")
local probe = require("wraith.manager.store_probe")
local git = require("wraith.manager.store_git")
local verify = require("wraith.manager.store_verify")
local promote = require("wraith.manager.store_promote")
local publish = require("wraith.manager.store_publish")
local cleanup = require("wraith.manager.store_cleanup")
local M = {}

function M.start(plan, options, progress, completed)
  local work, confirmed, changed = nil, nil, wraith.json.array()
  local control = flow.new(function(result)
    if confirmed and result.state ~= "installed" then
      local warning = result.error or "cleanup was interrupted"
      result = wraith.json.decode(wraith.json.encode(confirmed))
      result.cleanup_warning = warning .. "; inspect manager.recover() for private leftovers"
    end
    result.workspace = work
    result.restart_required = result.state == "installed" and changed or wraith.json.array()
    completed(result)
  end)
  function control.confirmed()
    return confirmed
  end

  local candidate = wraith.json.decode(wraith.json.encode(plan.baseline))
  for _, name in ipairs(plan.removed) do candidate.packages[name] = nil end

  local finish = control.finish

  local function check(result)
    local message = result.error or "package preparation failed"
    if result.stderr and result.stderr ~= "" then message = message .. ": " .. result.stderr:sub(-8192) end
    assert(result.state == "completed", message)
    return result
  end

  local function published(result)
    if result.state ~= "installed" then finish(result); return end
    confirmed = result
    confirmed.restart_required = changed
    progress("cleaning", nil, "remove this transaction's private files")
    control.begin(function(callback) return cleanup.start(work, plan.action == "restore", callback) end, function(cleaned)
      if cleaned.state == "completed" then finish(confirmed) else finish(cleaned) end
    end)
  end

  local function commit()
    if plan.action == "restore" then
      progress("verifying", nil, "confirm the selected lock is unchanged")
      work.check_only = true
      control.begin(function(callback) return publish.commit(work, options, callback) end, function(result)
        if result.state == "installed" then
          result.lock, result.generation = plan.baseline, plan.baseline.generation
        end
        published(result)
      end, true)
      return
    end
    progress("publishing", nil, "publish the complete package graph")
    candidate.generation = work.generation
    control.begin(function(callback) return files.start("write", { work.candidate, wraith.json.encode(candidate), { overwrite = true } }, callback) end, function(written)
      check(written)
      control.begin(function(callback) return publish.commit(work, options, callback) end, function(result)
        if result.state == "installed" then result.lock = candidate end
        published(result)
      end, true)
    end)
  end

  local install
  install = function(index)
    local item = plan.packages[index]
    if not item then commit(); return end
    if item.source.kind == "local" then
      progress("verifying", item.name, "check the local development directory")
      control.begin(function(callback) return files.start("stat", { item.source.dir, { follow = true } }, callback) end, function(result)
        assert(check(result).result.type == "directory", "local package path must be a directory")
        install(index + 1)
      end)
      return
    end

    local function selected(result)
      result = check(result)
      candidate.packages[item.name] = { source = item.source, selector = item.selector, pin = item.pin, commit = result.commit }
      if item.previous ~= result.commit or result.reused == false then changed[#changed + 1] = item.name end
      install(index + 1)
    end

    local function fetch()
      progress("fetching", item.name, item.commit and "fetch the exact locked revision" or "resolve the selected remote ref")
      control.begin(function(callback) return git.prepare(item, work.stage .. "/" .. item.name, options, callback) end, function(result)
        result = check(result)
        local resolved = wraith.json.decode(wraith.json.encode(item))
        resolved.commit = result.commit
        progress("verifying", item.name, "publish or verify the immutable checkout")
        control.begin(function(callback) return promote.install(resolved, result.path, options.root, options, callback) end, selected)
      end)
    end

    local function choose()
      if not item.commit then fetch(); return end
      control.begin(function(callback) return probe.object(options.root, item.name, item.commit, callback) end, function(found)
        found = check(found)
        if not found.exists then fetch(); return end
        progress("verifying", item.name, "verify the existing locked checkout")
        control.begin(function(callback) return verify.check(item, found.path, options, callback) end, selected)
      end)
    end

    if item.previous then
      progress("verifying", item.name, "preserve changes in the current installation")
      control.begin(function(callback) return probe.object(options.root, item.name, item.previous, callback) end, function(found)
        found = check(found)
        if not found.exists then choose(); return end
        local previous = wraith.json.decode(wraith.json.encode(item))
        previous.commit = item.previous
        control.begin(function(callback) return verify.check(previous, found.path, options, callback) end, function(result) check(result); choose() end)
      end)
    else
      choose()
    end
  end

  progress("preparing", nil, "check tools and reserve private transaction files")
  control.begin(function(callback) return workspace.prepare(options, plan.baseline_raw, callback) end, function(result)
    work = check(result).workspace
    install(1)
  end)
  return control
end

return M
