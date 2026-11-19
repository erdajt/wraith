return {
  setup = function(options)
    local buffer, process
    local function refresh()
      if process then return end
      if not buffer then
        buffer = wraith.buffers.create({ name = "changed files", lines = { "reading git status" } })
        wraith.windows.open(buffer, { position = "right", focus = true, title = "changed files" })
      end
      process = wraith.process.start({
        argv = { "git", "status", "--porcelain=v1", "-z" },
        cwd = options.directory or wraith.session.get().directory,
        timeout_ms = 10000,
        max_output_bytes = 65536,
      }, function(ctx)
        if not ctx.resource.final then return end
        local id = process
        process = nil
        local output = wraith.process.output(id, { max_bytes = 65536 })
        local lines = { "observed git changes" }
        if ctx.data.state ~= "completed" or (ctx.data.result and ctx.data.result.exit_code ~= 0) then
          local failure = wraith.process.output(id, { stream = "stderr", max_bytes = 4096 })
          lines[#lines + 1] = wraith.ui.decode(failure.data):gsub("[\r\n]", " ")
        else
          for entry in output.data:gmatch("([^%z]+)%z") do
            lines[#lines + 1] = wraith.ui.decode(entry):gsub("[\r\n]", " ")
          end
          if #lines == 1 then lines[#lines + 1] = "working tree clean" end
          if output.truncated or output.more then lines[#lines + 1] = "output truncated; narrow the repository query" end
        end
        wraith.resource.release(id)
        if not pcall(wraith.buffers.get, buffer) then buffer = nil; return end
        wraith.buffers.set_lines(buffer, 0, -1, lines)
        wraith.events.emit("files.refreshed", { buffer = buffer })
      end)
    end

    wraith.commands.create("changed-files", refresh, "inspect git working tree changes")
    wraith.events.on("jobs_changed", function() if buffer then refresh() end end)
  end,
}
