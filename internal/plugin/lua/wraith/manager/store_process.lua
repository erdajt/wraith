local wait = require("wraith.manager.store_wait")
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
  local control, finished, started = {}, false, false

  local function finish(status)
    if finished then return end
    finished = true
    local function output(stream)
      local ok, value = pcall(wraith.process.output, control.id, { stream = stream, max_bytes = 65536 })
      if ok then return value end
      if not started and (status.state == "cancelled" or status.state == "failed") and tostring(value):find("process.output: process has not started", 1, true) then
        return { data = "", truncated = false }
      end
      error(value)
    end
    local stdout, stderr = output("stdout"), output("stderr")
    completed({
      state = status.state, error = status.error, exit_code = status.result and status.result.exit_code,
      stdout = stdout.data, stderr = stderr.data, truncated = stdout.truncated or stderr.truncated,
    })
  end

  control.id = wraith.process.start({
    argv = argv, cwd = options.cwd, clear_env = true, env = M.environment(options.git_env),
    timeout_ms = options.timeout_ms or 300000, max_output_bytes = 65536,
  }, function(ctx)
    if ctx.resource.kind == "started" then started = true end
    if not ctx.resource.final then return end
    wait.finish(control.id, finish)
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
