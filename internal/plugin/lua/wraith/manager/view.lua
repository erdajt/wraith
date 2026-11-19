local M = {}

local function text(value, limit)
  value = tostring(value or "")
  local clipped = #value > limit
  value = wraith.ui.decode(value:sub(1, limit)):gsub("[\r\n\t]", " ")
  return clipped and value .. "..." or value
end

local function source(item)
  local value = item.source
  if type(value) ~= "table" then return text(value, 240) end
  return text(value.url or value.dir, 240)
end

function M.new(manager, options)
  options = options or {}
  for key in pairs(options) do assert(key == "position" or key == "keys", "unknown manager ui option: " .. tostring(key)) end
  local position = options.position or "right"
  assert(position == "right" or position == "left" or position == "above" or position == "below", "invalid manager window position")
  local keys = { refresh = "r", install = "i", update = "u", restore = "s", cancel = "x", close = "q", inspect = "<Enter>", confirm = "<Enter>", load = "l", unload = "d" }
  for action, key in pairs(options.keys or {}) do
    assert(keys[action] ~= nil, "unknown manager binding: " .. tostring(action))
    assert(key == false or type(key) == "string" and #key > 0 and #key <= 128, "invalid manager binding: " .. action)
    keys[action] = key
  end
  local used, identities = {}, {}
  for action, key in pairs(keys) do
    if key then
      wraith.keymap.validate("buffer.normal", key)
      local identity = table.concat(wraith.keymap.parse(key), "\0")
      identities[action] = identity
      if action ~= "confirm" then
        assert(not used[identity], "duplicate manager binding: " .. key)
        used[identity] = action
      end
    end
  end
  assert(keys.confirm and identities.confirm ~= identities.close, "plan confirmation requires a distinct binding from close")

  local buffer, window, detail_buffer, detail_window
  local rows, api = {}, {}
  local function close_detail()
    if detail_window and pcall(wraith.windows.get, detail_window) then wraith.windows.close(detail_window) end
  end
  local function details(title, lines, accept)
    if detail_buffer and pcall(wraith.buffers.get, detail_buffer) then
      wraith.buffers.set_lines(detail_buffer, 0, -1, lines)
      wraith.buffers.del_keymap(detail_buffer, "buffer.normal", keys.confirm)
    else
      detail_buffer = wraith.buffers.create({ name = "package details", lines = lines })
    end
    if keys.close then wraith.buffers.set_keymap(detail_buffer, "buffer.normal", keys.close, close_detail, "close package details") end
    if accept then
      wraith.buffers.set_keymap(detail_buffer, "buffer.normal", keys.confirm, accept, "confirm package plan")
      local _, confirm_end = keys.confirm:gsub("[^\128-\191]", "")
      local spans = { { start = { line = 1, column = 0 }, ["end"] = { line = 1, column = confirm_end }, style = { foreground = "accent" } } }
      if keys.close then
        local _, close_start = (keys.confirm .. " confirm · "):gsub("[^\128-\191]", "")
        local _, close_length = keys.close:gsub("[^\128-\191]", "")
        spans[#spans + 1] = { start = { line = 1, column = close_start }, ["end"] = { line = 1, column = close_start + close_length }, style = { foreground = "hint_quit" } }
      end
      wraith.buffers.set_spans(detail_buffer, spans)
    else
      wraith.buffers.set_spans(detail_buffer, {})
    end
    if detail_window and pcall(wraith.windows.get, detail_window) then
      wraith.windows.configure(detail_window, { title = title })
      wraith.windows.focus(detail_window)
    else
      detail_window = wraith.windows.open(detail_buffer, { position = "below", focus = true, title = title })
    end
  end
  local function selected(ctx)
    if not buffer or not ctx.input then return end
    local position, row = ctx.input.cursor, 1
    for _, line in ipairs(wraith.buffers.get(buffer).lines) do
      local _, count = line:gsub("[^\128-\191]", "")
      if position <= count then break end
      position, row = position - count - 1, row + 1
    end
    return rows[row - 3]
  end
  local function plan(action, item)
    local result = manager.plan(action, { item.name })
    local summary = result.summary
    local lines = { action .. " plan", keys.confirm .. " confirm · " .. (keys.close or "unbound") .. " close", "" }
    for _, entry in ipairs(summary.packages) do
      lines[#lines + 1] = text(entry.name, 128) .. " · " .. text(entry.state, 64)
      lines[#lines + 1] = "  source: " .. source(entry)
      lines[#lines + 1] = "  revision: " .. text(entry.commit or (entry.selector and (entry.selector.value or entry.selector.kind)), 255)
      if entry.previous then lines[#lines + 1] = "  previous: " .. text(entry.previous, 64) end
      if entry.pin then lines[#lines + 1] = "  pinned" end
    end
    for _, name in ipairs(summary.pruned or {}) do lines[#lines + 1] = "remove lock entry: " .. text(name, 128) .. " (files retained)" end
    lines[#lines + 1] = "installed changes take effect after reload; local directories are never modified"
    details("package plan", lines, function()
      wraith.ui.select({ { id = "apply", label = "apply reviewed plan" }, { id = "cancel", label = "cancel" } }, { title = action .. " packages" }, function(ctx)
        if ctx.data.cancelled or ctx.data.item_id ~= "apply" then return end
        manager.execute(result.id)
        close_detail()
        api.refresh()
      end)
    end)
  end
  local function bind()
    local actions = {
      refresh = function() api.refresh() end,
      close = function() api.close() end,
      cancel = function() manager.cancel(); api.refresh() end,
    }
    for _, action in ipairs({ "install", "update", "restore", "load", "unload", "inspect" }) do
      local operation = action
      actions[action] = function(ctx)
        local item = selected(ctx)
        if not item then wraith.notify("select a package row"); return end
        if operation == "inspect" then
          local lines = { text(item.name, 128), "source: " .. source(item), "path: " .. text(item.path, 1024), "state: " .. text(item.state, 128), "installed commit: " .. text(item.commit, 64), "loaded commit: " .. text(item.loaded_commit, 64), "loaded: " .. tostring(item.loaded == true), "pinned: " .. tostring(item.pin == true) }
          if item.error then lines[#lines + 1] = "error: " .. text(item.error, 4096) end
          if item.restart_required then lines[#lines + 1] = "reload required" end
          details("package details", lines)
        elseif operation == "load" or operation == "unload" then
          manager[operation](item.name)
          api.refresh()
        else
          plan(operation, item)
        end
      end
    end
    for action, key in pairs(keys) do
      if key and action ~= "confirm" then wraith.buffers.set_keymap(buffer, "buffer.normal", key, actions[action], action .. " package") end
    end
  end

  function api.refresh()
    if not buffer then return end
    if not pcall(wraith.buffers.get, buffer) then buffer, window = nil, nil; return end
    local status = manager.status()
    rows = status.packages
    local operation = status.operation
    local lines = { "packages" .. (operation and " · " .. text(operation.state, 128) or ""), "", "" }
    local spans, column = {}, 0
    for _, item in ipairs({ { "close", "hint_quit" }, { "refresh", "hint_refresh" }, { "cancel", "hint_cancel" }, { "install", "accent" }, { "update", "accent" }, { "restore", "accent" }, { "load", "accent" }, { "unload", "accent" }, { "inspect", "accent" } }) do
      local key = keys[item[1]]
      if key then
        local label = text(key, 128) .. " " .. item[1] .. "  "
        local _, length = label:gsub("[^\128-\191]", "")
        spans[#spans + 1] = { start = { line = 1, column = column }, ["end"] = { line = 1, column = column + length - 2 }, style = { foreground = item[2] } }
        lines[2], column = lines[2] .. label, column + length
      end
    end
    for _, item in ipairs(rows) do
      local state = item.loaded and "loaded" or item.state or (item.installed and "installed" or "missing")
      if type(item.source) == "table" and item.source.kind == "local" then state = state .. " · local" end
      if item.pin then state = state .. " · pinned" end
      if item.commit then state = state .. " · installed " .. text(item.commit, 8) end
      if item.loaded_commit then state = state .. " · loaded " .. text(item.loaded_commit, 8) end
      if item.restart_required then state = state .. " · reload required" end
      if item.error then state = state .. " · " .. text(item.error, 240) end
      lines[#lines + 1] = text(item.name, 128) .. " · " .. state .. " · " .. source(item)
    end
    if #rows == 0 then lines[#lines + 1] = "no packages configured" end
    if operation and operation.error then lines[#lines + 1] = "operation: " .. text(operation.error, 2048) end
    wraith.buffers.set_lines(buffer, 0, -1, lines)
    wraith.buffers.set_spans(buffer, spans)
  end
  function api.open()
    if buffer and not pcall(wraith.buffers.get, buffer) then buffer, window = nil, nil end
    if not buffer then
      buffer = wraith.buffers.create({ name = "packages", lines = { "packages" } })
      bind()
    end
    api.refresh()
    if window and pcall(wraith.windows.get, window) then
      wraith.windows.focus(window)
    else
      window = wraith.windows.open(buffer, { position = position, focus = true, title = "packages" })
    end
  end
  function api.close()
    close_detail()
    if window and pcall(wraith.windows.get, window) then wraith.windows.close(window) end
  end
  return api
end

return M
