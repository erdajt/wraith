local descriptor = require("wraith.manager.spec").descriptor
local options = require("wraith.manager.store_options")
local inspect = require("wraith.manager.store_inspect")
local planner = require("wraith.manager.store_plan")
local execute = require("wraith.manager.store_execute")
local copy = require("wraith.manager.spec_fields").copy
local M = {}

function M.new(specs, settings)
  local configured, config = {}, options.normalize(settings)
  for _, spec in ipairs(specs) do configured[#configured + 1] = descriptor(spec) end
  assert(#configured <= 63, "package graph exceeds 63 entries")
  local api, plans, order, serial, active, latest = {}, {}, {}, 0, nil, nil
  local identity = tostring({}):gsub("[^%w]", "")

  local function notify(callback, value)
    if not callback then return end
    local ok, err = pcall(callback, copy(value))
    if not ok then pcall(wraith.notify, "package observer: " .. wraith.ui.decode(tostring(err):sub(1, 2048))) end
  end

  local function reconcile()
    if not active or not active.control then return end
    local id = active.control.resource()
    if not id then return end
    local ok, status = pcall(wraith.resource.status, id)
    if ok and not status.callback_error then return end
    local confirmed = active.control.confirmed()
    pcall(active.control.abandon)
    local reason = ok and tostring(status.callback_error) or "package operation was discarded"
    if confirmed then
      latest.state, latest.generation = "installed", confirmed.generation
      latest.restart_required = copy(confirmed.restart_required)
      latest.message = confirmed.message or latest.message
      latest.cleanup_warning = reason .. "; inspect manager.recover() for private leftovers"
    else
      latest.state, latest.error = "failed", reason .. "; create a fresh plan"
    end
    local callback = active.completed
    active = nil
    notify(callback, latest)
  end

  function api.inspect()
    return inspect.read(configured, config)
  end

  function api.plan(action, names)
    reconcile()
    assert(not active, "a package operation is already running")
    local _, baseline, raw = inspect.read(configured, config)
    local plan = planner.create(configured, baseline, action, names)
    plan.baseline_raw = raw
    serial = serial + 1
    local id = identity .. ":plan:" .. serial
    plans[id], order[#order + 1] = plan, id
    while #order > 16 do plans[table.remove(order, 1)] = nil end
    return id, copy({ id = id, action = action, packages = plan.packages, pruned = plan.removed })
  end

  function api.execute(id, observer, completed)
    reconcile()
    assert(not active, "a package operation is already running")
    assert(observer == nil or type(observer) == "function", "package observer must be a function")
    assert(completed == nil or type(completed) == "function", "package completion must be a function")
    local plan = assert(plans[id], "unknown or expired package plan")
    plans[id] = nil
    serial = serial + 1
    local status = { id = identity .. ":operation:" .. serial, state = "preparing", phase = "preparing", restart_required = wraith.json.array() }
    latest = status
    local operation = { completed = completed, cancelled = false }
    active = operation
    local function progress(phase, name, message)
      if active ~= operation then return end
      status.state, status.phase, status.name, status.message = phase, phase, name, message
      notify(observer, status)
    end
    local run_options = copy(config)
    run_options.progress = function(resource)
      if active ~= operation then return end
      local ok, output = pcall(wraith.process.output, resource, { stream = "stderr", max_bytes = 8192 })
      if ok then
        status.log, status.log_truncated = output.data, output.truncated
        notify(observer, status)
      end
    end
    local ok, control = pcall(execute.start, plan, run_options, progress, function(result)
      if active ~= operation then return end
      status.state, status.error, status.code = result.state, result.error, result.code
      status.message = result.message or status.message
      status.restart_required = result.restart_required or wraith.json.array()
      status.generation, status.cleanup_warning = result.generation, result.cleanup_warning
      active = nil
      notify(observer, status)
      notify(completed, status)
    end)
    if not ok then
      active = nil
      status.state, status.error = "failed", tostring(control)
      notify(completed, status)
      error(control)
    end
    operation.control = control
    if operation.cancelled then control.cancel() end
    return status.id
  end

  function api.cancel(id)
    reconcile()
    if not active then
      assert(id == nil or latest and latest.id == id, "unknown package operation")
      return false
    end
    assert(id == nil or latest.id == id, "unknown package operation")
    active.cancelled = true
    if active.control then active.control.cancel() end
    return true
  end

  function api.status()
    reconcile()
    return latest and copy(latest) or nil
  end

  function api.recover()
    return require("wraith.manager.store_recover").inspect(config)
  end

  return api
end

return M
