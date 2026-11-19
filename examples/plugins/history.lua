return {
  setup = function(options)
    local buffer, window, rows, records = nil, nil, {}, {}
    local output_buffer, output_window
    local loading, dirty, stopped = false, false, false
    local maximum = options.max_records or 200
    assert(type(maximum) == "number" and maximum >= 1 and maximum <= 1000 and maximum % 1 == 0, "max_records must be 1 through 1000")

    local function render(status)
      rows = {}
      for _, record in pairs(records) do rows[#rows + 1] = record end
      table.sort(rows, function(a, b) return a.started > b.started end)
      for index = #rows, maximum + 1, -1 do
        records[rows[index].id] = nil
        table.remove(rows, index)
      end
      local lines = { status .. " · enter output · r refresh · / search" }
      for _, record in ipairs(rows) do
        local command = wraith.ui.decode(record.command):gsub("[\r\n]", " ")
        lines[#lines + 1] = record.status .. " · " .. command
      end
      wraith.buffers.set_lines(buffer, 0, -1, lines)
    end

    local refresh
    local function page(cursor)
      wraith.jobs.list({ cursor = cursor, limit = 40 }, function(ctx)
        if not ctx.data.ok then
          loading = false
          render(ctx.data.error.message)
          return
        end
        local result = ctx.data.result
        for _, record in ipairs(result.page.items) do records[record.id] = record end
        for _, record in ipairs(result.active) do records[record.id] = record end
        render(result.page.complete and "command history" or "loading history")
        if not result.page.complete and #rows < maximum then
          page(result.page.next)
          return
        end
        loading = false
        if not result.page.complete then wraith.workspace.cancel(ctx.data.id) end
        if #rows >= maximum then render("history limit reached; increase max_records to load more") end
        if #result.page.errors > 0 then render("history contains unreadable records") end
        if dirty then dirty = false; refresh(); return end
        wraith.events.emit("history.loaded", { buffer = buffer, count = #rows })
      end)
    end

    refresh = function(context)
      if stopped then return end
      if buffer and not pcall(wraith.buffers.get, buffer) then buffer = nil end
      if not buffer then
        buffer = wraith.buffers.create({ name = "command history", lines = { "loading history" } })
        wraith.buffers.set_keymap(buffer, "buffer.normal", "r", refresh, "refresh command history")
        wraith.buffers.set_keymap(buffer, "buffer.normal", "<Enter>", function(ctx)
          local position, row = ctx.input.cursor, 0
          for _, line in ipairs(wraith.buffers.get(buffer).lines) do
            local _, count = line:gsub("[^\128-\191]", "")
            if position <= count then break end
            position, row = position - count - 1, row + 1
          end
          local record = rows[row]
          if not record then return end
          wraith.jobs.output({ id = record.id, limit = 100 }, function(ctx)
            if not ctx.data.ok then wraith.notify(ctx.data.error.message); return end
            local lines = { wraith.ui.decode(record.command):gsub("[\r\n]", " "), "status: " .. ctx.data.result.record.status, "" }
            for _, line in ipairs(ctx.data.result.page.lines) do lines[#lines + 1] = line.text end
            if (ctx.data.result.page.next or 0) > 0 then lines[#lines + 1] = "next line: " .. ctx.data.result.page.next end
            if output_buffer and pcall(wraith.buffers.get, output_buffer) then
              wraith.buffers.set_lines(output_buffer, 0, -1, lines)
            else
              output_buffer = wraith.buffers.create({ name = "command output", lines = lines })
            end
            if output_window and pcall(wraith.windows.get, output_window) then
              wraith.windows.configure(output_window, { title = record.id })
              wraith.windows.focus(output_window)
            else
              output_window = wraith.windows.open(output_buffer, { position = "below", focus = true, title = record.id })
            end
            wraith.events.emit("history.output", { id = record.id, buffer = output_buffer })
          end)
        end, "open command output")
      end
      if context then
        if window and pcall(wraith.windows.get, window) then
          wraith.windows.focus(window)
        else
          window = wraith.windows.open(buffer, { position = "right", focus = true, title = "command history" })
        end
      end
      if loading then dirty = true; return end
      loading, records = true, {}
      page(nil)
    end
    wraith.commands.create("command-history", refresh, "browse native command history")
    wraith.events.on("jobs_changed", function() if buffer then refresh() end end)
    return function() stopped = true end
  end,
}
