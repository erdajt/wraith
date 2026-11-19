local M = {}
local inherited = {
  "PATH", "HOME", "SSH_AUTH_SOCK", "SSH_AGENT_PID", "HTTP_PROXY", "HTTPS_PROXY",
  "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "all_proxy", "no_proxy",
  "SSL_CERT_FILE", "SSL_CERT_DIR",
}
local authentication = { GIT_ASKPASS = true, GIT_SSH = true, GIT_SSH_COMMAND = true, SSH_ASKPASS = true }

function M.environment(overrides)
  local env = { LC_ALL = "C", GIT_TERMINAL_PROMPT = "0", GIT_CONFIG_NOSYSTEM = "1", GIT_CONFIG_GLOBAL = "/dev/null" }
  for _, key in ipairs(inherited) do env[key] = os.getenv(key) end
  for key, value in pairs(overrides or {}) do
    assert(type(key) == "string" and authentication[key], "unsupported package authentication variable: " .. tostring(key))
    assert(type(value) == "string" and not value:find("%z"), "invalid package authentication value")
    env[key] = value
  end
  return env
end

function M.start(argv, options, completed)
  assert(type(completed) == "function", "package process callback is required")
  options = options or {}
  local control, finished, poll = {}, false, nil

  local function finish(status)
    if finished then return end
    finished = true
    local stdout = wraith.process.output(control.id, { stream = "stdout", max_bytes = 65536 })
    local stderr = wraith.process.output(control.id, { stream = "stderr", max_bytes = 65536 })
    if poll then
      wraith.resource.cancel(poll)
      wraith.resource.release(poll)
      poll = nil
    end
    wraith.resource.release(control.id)
    completed({
      state = status.state, error = status.error, exit_code = status.result and status.result.exit_code,
      stdout = stdout.data, stderr = stderr.data, truncated = stdout.truncated or stderr.truncated,
    })
  end

  control.id = wraith.process.start({
    argv = argv, cwd = options.cwd, clear_env = true, env = M.environment(options.git_env),
    timeout_ms = options.timeout_ms or 300000, max_output_bytes = 65536,
  }, function(ctx)
    if not ctx.resource.final then return end
    local status = wraith.resource.status(control.id)
    if status.done then finish(status); return end
    poll = wraith.timer.start(20, function()
      local current = wraith.resource.status(control.id)
      if current.done then finish(current) end
    end, { repeat_ms = 20 })
  end)

  function control.cancel()
    if not finished then wraith.resource.cancel(control.id) end
  end

  function control.output(stream)
    assert(not finished, "package process has finished")
    return wraith.process.output(control.id, { stream = stream or "stderr", max_bytes = 65536 })
  end

  return control
end

return M
