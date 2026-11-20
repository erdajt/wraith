local manager = require("wraith.manager")

wraith.config.setup({ theme = "nord", execution = { mode = "agent" } })
wraith.commands.create("review", function(ctx)
  local text = ctx.argument .. ctx.session.directory
  wraith.prompt.set(text)
  wraith.input.set_mode("normal")
  wraith.notify(text)
end, "prepare review", { arguments = true })

wraith.keymap.set("prompt.normal", "<leader>r", "command.review")
wraith.keymap.set("chat.normal", "<leader>v", function(ctx)
  wraith.notify(ctx.target)
end)
wraith.events.on("ready", function(ctx)
  wraith.notify(ctx.session.id)
end)

---@type integer
local version = wraith.api_version
---@type string
local root = wraith.paths.stdpath("config")
---@type string[]
local tokens = wraith.keymap.parse("<C-w>v")
---@type string
local theme = wraith.config.get().theme
print(version, root, tokens[1], theme)

manager.setup({
  {
    name = "review",
    dir = "lua/plugins/review",
    main = "review",
    cmd = "review.open",
    opts = { title = "review" },
  },
}, {
  dev = {
    path = function(spec)
      return "~/projects/" .. spec.name
    end,
    patterns = { "owner/" },
    fallback = true,
  },
})

local buffer = wraith.buffers.create({ name = "review", editable = true, lines = { "ready" } })
local pane = wraith.windows.open(buffer, { kind = "split", position = "right" })
wraith.buffers.set_lines(buffer, 0, -1, { "updated" }, wraith.buffers.get(buffer).revision)
wraith.windows.configure(pane, { cursor = { line = 0, column = 2 } })
wraith.regions.create({ slot = "header", lines = { "review" } })
wraith.ui.select({ "review", "close" }, { title = "action" }, function(ctx)
  if ctx.data and ctx.data.index then
    wraith.notify(tostring(ctx.data.index))
  end
end)

wraith.fs.read_async("review.txt", { max_bytes = 1024 }, function(ctx)
  if ctx.data and ctx.data.result then
    wraith.notify(ctx.data.result.data)
  end
end)
---@type integer
local size = wraith.fs.stat("review.txt").size
local metadata = wraith.fs.stat("review.txt", { missing_ok = true })
if metadata.exists then
  size = metadata.size
else
  print(metadata.path)
end
print(size)
wraith.fs.stat_async("review.txt", { missing_ok = true }, function(ctx)
  if ctx.data and ctx.data.result and ctx.data.result.exists then
    wraith.notify(tostring(ctx.data.result.size))
  end
end)
wraith.process.start({ argv = { "git", "status", "--short" }, mode = "capture" }, function(ctx)
  if ctx.resource then
    wraith.resource.release(ctx.resource.id)
  end
end)
wraith.http.request({ url = "https://example.com", timeout_ms = 1000 }, function(ctx)
  if ctx.data and ctx.data.result then
    wraith.notify(tostring(ctx.data.result.status))
  end
end)
wraith.jobs.output({ id = "0123456789abcdef01234567", pattern = "error", limit = 20 }, function(ctx)
  if ctx.data and ctx.data.result then
    wraith.notify(ctx.data.result.page.lines[1].text)
  end
end)
wraith.sessions.get({}, function(ctx)
  if ctx.data and ctx.data.result then
    wraith.notify(ctx.data.result.title)
  end
end)
wraith.environment.list({}, function(ctx)
  if ctx.data and ctx.data.result then
    wraith.notify(ctx.data.result.items[1].description)
  end
end)
wraith.hooks.add("before_command", function(ctx)
  return { command = ctx.command, decision = "allow" }
end)
wraith.hooks.add("before_request", function(ctx)
  return { request = ctx.request }
end)
wraith.tools.register("review.summary", {
  parameters = { type = "object", properties = { subject = { type = "string" } } },
  run = function(ctx)
    if ctx.invocation then
      wraith.invocation.finish(ctx.invocation.id, { summary = "ready" })
    end
  end,
})
local plan = manager.plan("install", { "review" })
---@type string
local handle = plan.id
print(handle, manager.status().packages[1].loaded)

---@type Wraith.PackageOperation[]
local operations = {
  { id = "1", phase = "preparing", state = "preparing", restart_required = {} },
  { id = "2", phase = "fetching", state = "fetching", restart_required = {} },
  { id = "3", phase = "verifying", state = "verifying", restart_required = {} },
  { id = "4", phase = "publishing", state = "publishing", restart_required = {} },
  { id = "5", phase = "cleaning", state = "cleaning", restart_required = {} },
  { id = "6", phase = "cleaning", state = "installed", restart_required = { "review" } },
  { id = "7", phase = "fetching", state = "failed", restart_required = {}, error = "fetch failed" },
  { id = "8", phase = "fetching", state = "cancelled", restart_required = {} },
  { id = "9", phase = "publishing", state = "conflict", restart_required = {}, error = "lock changed" },
  { id = "10", phase = "publishing", state = "uncertain", restart_required = {}, error = "inspect lock" },
}
for _, operation in ipairs(operations) do
  if operation.state == "conflict" or operation.state == "uncertain" then
    print(operation.error, manager.recover().root)
  end
end

---@type Wraith.ModuleCheck
local verify_source = function(name, phase, proof, executed)
  if phase == "before" then
    return { name = name, executed = executed }
  elseif phase == "after" then
    assert(proof.name == name and proof.executed == executed)
  else
    assert(phase == "validate" and proof == nil and not executed)
  end
end
---@type Wraith.ModuleResolver
local resolve_source = function(name)
  if name ~= "review" then
    return nil
  end
  return {
    plugin = "plugins.review",
    prepare = function(module_name)
      assert(wraith.capabilities.module_preparation)
      wraith.plugins.load("initializers." .. module_name)
    end,
    activate = function(module_name)
      wraith.plugins.load("plugins." .. module_name)
    end,
    check = verify_source,
  }
end
local provider_id = wraith.modules.provider(resolve_source)
wraith.modules.remove_provider(provider_id)
wraith.plugins.unload_many({ "plugins.review", "plugins.helper" })
wraith.plugins.unload_many({})
wraith.events.replay_ready(wraith.events.listeners("ready"))
local plugin_state = wraith.plugins.state("plugins.review")
if plugin_state == "loading" then
  print("initializing")
elseif plugin_state == "loaded" then
  print("initialized")
end
