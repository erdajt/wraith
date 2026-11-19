local copy = require("wraith.manager.spec_fields").copy
local M, state = {}, nil
local runtime = "wraith.manager.runtime"

local function fresh_listeners(event, before)
  local seen, result = {}, {}
  for _, id in ipairs(before) do seen[id] = true end
  for _, id in ipairs(wraith.events.listeners(event)) do
    if not seen[id] then result[#result + 1] = id end
  end
  return result
end

function M.setup()
  state = nil
end

function M.configure(graph, options)
  assert(state == nil, "package manager is already configured")
  local descriptors = {}
  for _, spec in ipairs(graph.ordered) do
    descriptors[#descriptors + 1] = require("wraith.manager.spec").descriptor({name=spec.name, source=spec.source, selector=spec.selector, pin=spec.pin, dependencies=spec.dependencies})
  end
  local store = require("wraith.manager.store").new(descriptors, options.storage)
  local rows, paths, selected = store.inspect(), {}, {}
  for _, row in ipairs(rows) do
    paths[row.name], selected[row.name] = row.path, copy(row)
  end
  local loader = require("wraith.manager.loader").new(graph, paths, runtime, selected)
  local facade = {}
  for _, method in ipairs({"status", "plan", "execute", "cancel", "load", "unload"}) do
    facade[method] = function(...) return wraith.plugins.call(runtime, method, ...) end
  end
  local view = require("wraith.manager.view").new(facade, options.ui)
  state = {graph=graph, store=store, loader=loader, selected=selected, view=view}
  loader.initialize()

  local events, event_names = {}, {}
  for _, spec in ipairs(graph.ordered) do
    for _, event in ipairs(spec.event) do
      if not events[event] then events[event] = {}; event_names[#event_names + 1] = event end
      events[event][#events[event] + 1] = spec.name
    end
  end
  table.sort(event_names)
  for _, event in ipairs(event_names) do
    wraith.events.on(event, function(ctx)
      local cohort, states = {}, loader.states()
      for _, name in ipairs(events[event]) do
        if states[name].managed and not states[name].loaded then
          cohort[#cohort + 1] = {name=name, identity=loader.identity(name)}
        end
      end
      if #cohort == 0 then return end
      local data, before = ctx.data, wraith.events.listeners(event)
      wraith.schedule(function() wraith.plugins.call(runtime, "on_event", event, data, before, cohort) end)
    end)
  end
  wraith.commands.create("packages", function() view.open() end, "manage lua packages")
  for _, method in ipairs({"install", "update", "restore", "load", "unload"}) do
    wraith.commands.create("packages." .. method, function(ctx)
      local name = ctx.argument:match("^%s*(.-)%s*$")
      assert(name ~= "", "choose a package name")
      wraith.plugins.call(runtime, method, name)
    end, method .. " a package", {arguments=true})
  end
  wraith.commands.create("packages.cancel", function() M.cancel() end, "cancel package operation")
  local function eager(index)
    while graph.ordered[index] and graph.ordered[index].lazy do index = index + 1 end
    local spec = graph.ordered[index]
    if not spec then return end
    wraith.schedule(function()
      local ok, err = pcall(wraith.plugins.call, runtime, "load", spec.name)
      if not ok then wraith.notify("package " .. spec.name .. ": " .. tostring(err)) end
      eager(index + 1)
    end)
  end
  eager(1)
end

function M.status()
  assert(state, "package manager is not configured")
  local rows, states = state.store.inspect(), state.loader.states()
  for _, row in ipairs(rows) do
    local chosen = state.selected[row.name]
    row.loaded, row.managed = states[row.name].loaded, states[row.name].managed
    row.installed = row.path ~= nil
    row.lazy = state.graph.by_name[row.name].lazy
    row.loaded_commit = row.loaded and chosen and chosen.commit or nil
    row.restart_required = not chosen or row.path ~= chosen.path or row.commit ~= chosen.commit
    if not row.managed then row.state = "unloaded" end
  end
  return {packages=rows, operation=state.store.status()}
end

function M.load(name)
  assert(state, "package manager is not configured")
  local before = wraith.events.listeners("ready")
  state.loader.load(name)
  local listeners = fresh_listeners("ready", before)
  if #listeners > 0 then wraith.events.dispatch("ready", nil, {listeners=listeners}) end
end

function M.trigger(name, kind, data)
  M.load(name)
  state.loader.trigger(name, kind, data)
end

function M.on_event(event, data, observed, cohort)
  assert(state, "package manager is not configured")
  local ready, selected, existing = wraith.events.listeners("ready"), {}, {}
  for _, id in ipairs(observed) do existing[id] = true end
  for _, member in ipairs(cohort) do
    if state.loader.identity(member.name) == member.identity then
      state.loader.load(member.name)
      selected[#selected + 1] = member.name
    end
  end
  local listeners = {}
  for _, id in ipairs(state.loader.listeners(selected, event)) do
    if not existing[id] then listeners[#listeners + 1] = id end
  end
  if #listeners > 0 then wraith.events.dispatch(event, data, {listeners=listeners}) end
  if event ~= "ready" then
    local added = fresh_listeners("ready", ready)
    if #added > 0 then wraith.events.dispatch("ready", nil, {listeners=added}) end
  end
end

function M.unload(name)
  assert(state, "package manager is not configured")
  state.loader.unload(name)
end

function M.unload_all()
  assert(state, "package manager is not configured")
  state.store.cancel()
  state.loader.unload_all()
end

function M.plan(action, names)
  assert(state, "package manager is not configured")
  local id, summary = state.store.plan(action, names)
  return {id=id, summary=summary}
end

function M.execute(id)
  assert(state, "package manager is not configured")
  return state.store.execute(id, function() state.view.refresh() end, function(result)
    state.view.refresh()
    if result.error then wraith.notify("packages: " .. result.error) end
  end)
end

function M.cancel(id)
  assert(state, "package manager is not configured")
  return state.store.cancel(id)
end

function M.recover()
  assert(state, "package manager is not configured")
  return state.store.recover()
end

for _, action in ipairs({"install", "update", "restore"}) do
  M[action] = function(names)
    if type(names) == "string" then names = {names} end
    return M.execute(M.plan(action, names).id)
  end
end

function M.open()
  assert(state, "package manager is not configured")
  state.view.open()
end

return M
