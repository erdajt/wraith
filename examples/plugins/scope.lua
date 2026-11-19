return {
  setup = function(options)
    assert(type(options.path) == "string", "scope requires a policy file")
    local buffer, window
    local policy, failure, revision = nil, "policy has not been loaded", 0
    local function reload()
      policy, failure = nil, "policy is loading"
      revision = revision + 1
      local current = revision
      wraith.fs.read_async(options.path, { max_bytes = 65536 }, function(ctx)
        wraith.resource.release(ctx.resource.id)
        if current ~= revision then return end
        if ctx.data.state ~= "completed" then
          failure = ctx.data.error or "policy read failed"
        elseif ctx.data.result.truncated then
          failure = "policy exceeds 65536 bytes"
        else
          local ok, decoded = pcall(wraith.json.decode, ctx.data.result.data)
          if not ok or type(decoded) ~= "table" or type(decoded.directory) ~= "string" or type(decoded.commands) ~= "table" then
            failure = "policy requires a directory and exact command list"
          else
            local valid = #decoded.commands > 0
            for _, command in ipairs(decoded.commands) do valid = valid and type(command) == "string" end
            if valid then policy, failure = decoded, nil else failure = "policy command list is invalid" end
          end
        end
        wraith.events.emit("scope.loaded", { ready = policy ~= nil, error = failure })
      end)
    end

    wraith.commands.create("scope-reload", reload, "reload the exact native command policy")
    wraith.commands.create("scope", function()
      local lines = { "native command policy" }
      if policy then
        lines[#lines + 1] = policy.directory
        for _, command in ipairs(policy.commands) do lines[#lines + 1] = command end
      else
        lines[#lines + 1] = failure
      end
      if buffer and pcall(wraith.buffers.get, buffer) then
        wraith.buffers.set_lines(buffer, 0, -1, lines)
      else
        buffer = wraith.buffers.create({ name = "scope", lines = lines })
      end
      if window and pcall(wraith.windows.get, window) then
        wraith.windows.focus(window)
      else
        window = wraith.windows.open(buffer, { position = "right", focus = true, title = "scope" })
      end
    end)
    wraith.hooks.add("before_command", function(ctx)
      if not policy then return { decision = "deny", reason = failure } end
      if ctx.command.directory ~= policy.directory then
        return { decision = "deny", reason = "command directory is outside policy" }
      end
      if policy.shell and ctx.command.shell ~= policy.shell then
        return { decision = "deny", reason = "shell is outside policy" }
      end
      for _, command in ipairs(policy.commands) do
        if command == ctx.command.command then return end
      end
      return { decision = "deny", reason = "command is not in the exact allowlist" }
    end)
    wraith.schedule(reload)
  end,
}
