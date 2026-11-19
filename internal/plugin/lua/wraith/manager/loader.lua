local copy = require("wraith.manager.spec_fields").copy
local M = {}
local wrappers = {}

local function loaded()
  local result = {}
  for _, name in ipairs(wraith.plugins.list()) do result[name] = true end
  return result
end

function M.new(graph, paths, runtime)
  local self, names, generation = {}, {}, 0
  local watched, event_names = {ready=true}, {"ready"}
  for _, spec in ipairs(graph.ordered) do
    for _, event in ipairs(spec.event) do
      if not watched[event] then watched[event] = true; event_names[#event_names + 1] = event end
    end
  end
  table.sort(event_names)
  local active = loaded()
  assert(#graph.ordered * 2 + #wraith.plugins.list() <= 128, "managed packages exceed available lifecycle capacity")
  for name in pairs(wrappers) do
    assert(not active[name], "package manager is already configured")
    package.preload[name], package.loaded[name], wrappers[name] = nil, nil, nil
  end
  for _, spec in ipairs(graph.ordered) do
    local prefix = "wraith.manager.package." .. spec.name
    names[spec.name] = {init=prefix .. ".init", activation=prefix .. ".active"}
    for _, module in pairs(names[spec.name]) do
      assert(not package.preload[module] and not package.loaded[module], "reserved package wrapper already exists: " .. module)
    end
  end

  for _, spec in ipairs(graph.ordered) do
    local identity = names[spec.name]
    package.preload[identity.init] = function()
      generation = generation + 1
      local init, cleanup, identity_token = {}, nil, tostring(generation)
      function init.identity() return identity_token end
      function init.arm()
        for _, command in ipairs(spec.cmd) do
          wraith.commands.create(command, function(ctx)
            wraith.plugins.call(runtime, "trigger", spec.name, "command", {name=command, argument=ctx.argument})
          end, "load " .. spec.name, {arguments=true})
        end
        for index, key in ipairs(spec.keys) do
          wraith.keymap.set(key.mode, key.keys, function(ctx)
            wraith.plugins.call(runtime, "trigger", spec.name, "key", {index=index, context=ctx})
          end, key.description)
        end
      end
      function init.disarm()
        for _, command in ipairs(spec.cmd) do wraith.commands.delete(command) end
        for _, key in ipairs(spec.keys) do wraith.keymap.del(key.mode, key.keys) end
      end
      function init.setup()
        cleanup = nil
        wraith.plugins.depend(runtime)
        if spec.init then cleanup = spec.init(copy(spec)) end
        assert(cleanup == nil or type(cleanup) == "function", "package init must return a cleanup function or nil")
        init.arm()
      end
      function init.teardown()
        local callback = cleanup
        cleanup = nil
        if callback then callback() end
      end
      return init
    end

    package.preload[identity.activation] = function()
      local activation, main, pending_cleanup, subscriptions = {}, nil, nil, {}
      function activation.setup()
        main, pending_cleanup = nil, nil
        local previous = {}
        for _, event in ipairs(event_names) do
          previous[event] = {}
          for _, id in ipairs(wraith.events.listeners(event)) do previous[event][id] = true end
        end
        wraith.plugins.depend(identity.init)
        for _, dependency in ipairs(spec.dependencies) do wraith.plugins.depend(names[dependency].activation) end
        local opts = type(spec.opts) == "table" and copy(spec.opts) or {}
        if type(spec.opts) == "function" then
          local returned = spec.opts(copy(spec), opts)
          if returned ~= nil then opts = returned end
        end
        assert(type(opts) == "table", "package opts must produce a table")
        local result
        if type(spec.config) == "function" then
          result = spec.config(copy(spec), opts)
        else
          main = require(spec.main)
          if spec.config ~= false and type(main) == "table" and main.setup ~= nil then
            assert(type(main.setup) == "function", "package setup must be a function")
            result = main.setup(opts)
          end
        end
        pending_cleanup = type(result) == "function" and result or nil
        for _, key in ipairs(spec.keys) do wraith.keymap.set(key.mode, key.keys, key.action, key.description) end
        for _, event in ipairs(event_names) do
          subscriptions[event] = {}
          for _, id in ipairs(wraith.events.listeners(event)) do
            if not previous[event][id] then subscriptions[event][#subscriptions[event] + 1] = id end
          end
        end
        if pending_cleanup then
          return function()
            local callback = pending_cleanup
            pending_cleanup = nil
            if callback then callback() end
          end
        end
      end
      function activation.teardown()
        local module = main
        main = nil
        local callback, errors = pending_cleanup, {}
        pending_cleanup = nil
        if callback then
          local ok, err = pcall(callback)
          if not ok then errors[#errors + 1] = tostring(err) end
        end
        if type(module) == "table" and type(module.teardown) == "function" then
          local ok, err = pcall(module.teardown)
          if not ok then errors[#errors + 1] = tostring(err) end
        end
        if #errors > 0 then error(table.concat(errors, "; ")) end
      end
      function activation.listeners(event) return copy(subscriptions[event] or {}) end
      function activation.key(index, ctx)
        local key = assert(spec.keys[index], "unknown package key")
        if type(key.action) == "function" then return key.action(ctx) end
        wraith.action(key.action)
      end
      function activation.defer_key(index, ctx)
        wraith.schedule(function(current)
          assert(ctx.target == current.target, "lazy key target changed before activation completed")
          activation.key(index, ctx)
        end)
      end
      return activation
    end
    wrappers[identity.init], wrappers[identity.activation] = true, true
  end

  function self.initialize()
    for _, spec in ipairs(graph.ordered) do
      wraith.plugins.load(names[spec.name].init, {}, {dependency=false})
    end
  end
  function self.states()
    local active, result = loaded(), {}
    for name, identity in pairs(names) do
      result[name] = {loaded=active[identity.activation] == true, managed=active[identity.init] == true}
    end
    return result
  end
  function self.is_loaded(name)
    return names[name] ~= nil and loaded()[names[name].activation] == true
  end
  function self.is_managed(name)
    return names[name] ~= nil and loaded()[names[name].init] == true
  end
  function self.identity(name)
    if self.is_managed(name) then return wraith.plugins.call(names[name].init, "identity") end
  end
  function self.listeners(cohort, event)
    local selected, visited = {}, {}
    local function collect(name)
      if visited[name] or not self.is_loaded(name) then return end
      visited[name] = true
      for _, dependency in ipairs(graph.by_name[name].dependencies) do collect(dependency) end
      for _, id in ipairs(wraith.plugins.call(names[name].activation, "listeners", event)) do selected[id] = true end
    end
    for _, name in ipairs(cohort) do collect(name) end
    local result = {}
    for _, id in ipairs(wraith.events.listeners(event)) do if selected[id] then result[#result + 1] = id end end
    return result
  end
  function self.load(name)
    local spec = assert(graph.by_name[name], "unknown package: " .. tostring(name))
    assert(spec.enabled and names[name], "package is disabled: " .. name)
    if self.is_loaded(name) then return end
    local path = assert(paths[name], "package is not installed; install and reload: " .. name)
    if not self.is_managed(name) then
      package.loaded[names[name].init] = nil
      wraith.plugins.load(names[name].init, {}, {dependency=false})
    end
    for _, dependency in ipairs(spec.dependencies) do self.load(dependency) end
    wraith.runtimepath.append(path)
    wraith.plugins.call(names[name].init, "disarm")
    package.loaded[names[name].activation] = nil
    wraith.plugins.load(names[name].activation, {}, {dependency=false})
  end
  function self.trigger(name, kind, data)
    self.load(name)
    if kind == "command" then wraith.action("command." .. data.name, data.argument or "")
    elseif kind == "key" then wraith.plugins.call(names[name].activation, "defer_key", data.index, data.context)
    else error("unknown package trigger") end
  end
  function self.unload(name)
    assert(names[name], "unknown package: " .. tostring(name))
    if self.is_loaded(name) then wraith.plugins.unload(names[name].activation) end
    if self.is_managed(name) then wraith.plugins.unload(names[name].init) end
  end
  function self.unload_all()
    for index = #graph.ordered, 1, -1 do
      local name = graph.ordered[index].name
      if self.is_loaded(name) then wraith.plugins.unload(names[name].activation) end
    end
    for index = #graph.ordered, 1, -1 do
      local name = graph.ordered[index].name
      if self.is_managed(name) then wraith.plugins.unload(names[name].init) end
    end
  end
  return self
end

return M
