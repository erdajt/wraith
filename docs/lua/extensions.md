# request hooks, providers and model tools

[plugin setup](../plugins.md) · [managed services](services.md) · [buffers and views](surfaces.md) · [providers](../providers.md) · [execution](../execution.md) · [workspace queries](workspace.md) · [example packages](examples.md) · [package manager](packages.md)

`wraith.capabilities.hooks`, `providers`, `model_tools`, `invocations` and `workspace_queries` report these API families.

register extensions from `init.lua` or a package's `setup` function. runtime registrations become visible after their callback succeeds and the host accepts the new registry. failed setup, callback errors and rejected publications restore the previous registrations.

## request hooks

```lua
local hook = wraith.hooks.add("before_request", function(ctx)
  local request = ctx.request
  request.options.temperature = 0.2
  return { request = request }
end, { priority = 10, purposes = { "chat" } })

-- remove only a hook owned by this package
wraith.hooks.remove(hook)
```

hooks run in ascending priority, then registration order. priority ranges from `-10000` to `10000`. `purposes` defaults to `{ "chat" }`; opt into `"extract"` and `"audit"` for compaction work. request hooks run once before repair attempts. retries reuse the transformed base and add host-generated correction instructions. token estimation does not run hooks.

`ctx` contains immutable `kind`, `phase`, `purpose`, `id`, `session_id` and `directory`, plus the replaceable `request`:

| request field | value |
| --- | --- |
| `model` | nonempty model name |
| `messages` | array of `{ role, content, reference? }` |
| `think` | boolean, string or omitted |
| `options` | model options, including `num_ctx`, `num_predict`, `temperature` and `num_thread` |

return `nil` to leave the request unchanged. return `{ request = replacement }` to replace the complete mutable request. return `{ decision = "deny", reason = "..." }` to stop it. unknown fields, invalid types, errors and timeouts fail the request.

protected command evidence appears as an opaque `reference` with empty `content`. a hook may omit evidence; when keeping it, retain its original role and reference. references cannot be invented, duplicated or given replacement content. the host restores the original evidence and reapplies output-sharing rules, private-variable redaction, execution instructions and context limits after transformation. hooks cannot change the execution mode or the tool catalog through this DTO.

## command hooks

```lua
wraith.hooks.add("before_command", function(ctx)
  if ctx.command.directory ~= ctx.directory then
    return { decision = "deny", reason = "command directory changed" }
  end
  return { command = ctx.command }
end)
```

`ctx.phase` distinguishes preview from execution. the mutable command contains:

| field | value |
| --- | --- |
| `command` | complete shell command |
| `directory` | working directory |
| `shell` | executable shell |
| `environment` | ordered array of `NAME=value` entries |
| `timeout` | positive Go duration string, such as `"30s"` or `"1.5ms"` |
| `background` | boolean |

approval applies to that exact tuple. saving an edited command approves the saved text once. a subsequent hook transformation or hook registry change requires renewed approval before execution. a denial stops execution. hooks receive local resolved values as trusted code; provider requests still pass through privacy filtering.

hooks return data. they cannot call host actions, register callbacks, start managed work, open dialogs, mutate buffers or print notices. `wraith.json` remains available for value conversion. catching a forbidden call with `pcall` does not make the hook valid. raw `io`, `os` and other trusted Lua facilities still have local process authority; this restriction is not a security sandbox.

## custom providers

```lua
wraith.providers.register("local_example", {
  description = "example transport",
  models = function(ctx)
    wraith.invocation.finish(ctx.invocation.id, {
      { name = "example-model", size = 0 },
    })
  end,
  show = function(ctx)
    wraith.invocation.finish(ctx.invocation.id, {
      context_window = 32768,
      capabilities = { "completion" },
    })
  end,
  chat = function(ctx)
    local id = ctx.invocation.id
    wraith.invocation.emit(id, { content = "ready" })
    wraith.invocation.finish(id, { reason = "stop" })
  end,
})
```

select the registration through an explicit provider profile:

```lua
wraith.config.setup({
  providers = {
    local_example = { kind = "plugin", model = "example-model" },
  },
  provider = "local_example",
  model = "example-model",
})
```

model and context selections save a minimal `kind: plugin` profile in YAML when the profile exists only in Lua. unrelated Lua values, including credentials, are not copied. explicit values in `init.lua` still override YAML on reload; resumed sessions restore their saved model.

all three callbacks are required. `ctx.invocation` contains `id`, `kind`, `name` and `method`. `ctx.data` contains the method request. the host supplies a fresh workspace snapshot when dispatching the callback.

| callback | input | final result |
| --- | --- | --- |
| `models` | `{}` | array of `{ name, size }` |
| `show` | `{ model = "selected-model" }` | `{ context_window, capabilities, thinking? }` |
| `chat` | prepared request described below | final text, tool calls and usage metadata |

the chat request contains `model`, `messages`, `options`, `stream` and `keep_alive`, plus `think`, `tools` and `format` when applicable. messages use the native request shape; options use the fields listed under [request hooks](#request-hooks). tool entries contain `type = "function"` and `function = { name, description, parameters }`. transport callbacks receive the request after host instructions and privacy filtering.

discovery callbacks finish once without streaming. `models` returns an array, including `wraith.json.array({})` for no models; names must be unique and sizes nonnegative, with at most 1024 entries. `show` returns an object with nonnegative `context_window`, up to 32 capability strings and optional `thinking = { values = { ... }, default = ... }`; thinking values are boolean or text. null or a wrong container type is rejected.

chat emits `{ content = "...", thinking = "..." }` deltas. either field may be omitted. its final result may contain `content`, `thinking`, `calls`, `reason`, `prompt_tokens`, `output_tokens` and `duration_ns`. `calls` contains `{ name, arguments }` entries. `reason` is `"stop"` or `"length"`. chat results must be objects. at most one final call is accepted, with an object argument payload of at most 32 KiB; a `"length"` result cannot contain a call. total content and thinking are limited to 16 MiB across all deltas and the final result. usage counts and duration must be nonnegative. the host validates provider results before they enter the conversation pipeline.

use managed HTTP or process callbacks for transport work. return from the initial callback, then emit or finish from a later callback. do not block the Lua actor waiting for a callback that needs the same actor.

## model tools

```lua
wraith.tools.register("describe_workspace", {
  description = "describe a named workspace area",
  approval = "required",
  parameters = {
    type = "object",
    properties = { area = { type = "string", maxLength = 80 } },
    required = { "area" },
    additionalProperties = false,
  },
  run = function(ctx)
    wraith.invocation.finish(ctx.invocation.id, {
      area = ctx.data.arguments.area,
      description = "local workspace metadata",
    })
  end,
})
```

`approval` defaults to `"required"`. `"read_only"` is a declaration for host policy; current host policy requires approval for both values. native command and output tool names are reserved. model tools enter the agent tool catalog; blocks mode and compaction do not advertise them. the handler receives `{ arguments, job_id, directory }` in `ctx.data`. arguments contain resolved local values after approval. streamed and final results become output in the tracked job.

schemas use the [supported, bounded subset of JSON Schema draft 2020-12](tool-schemas.md). the root must describe an object. remote references, recursive references and unsupported keywords are rejected at registration. schemas and arguments are limited to 32 KiB each. use `wraith.json.array({})`, `wraith.json.object({})` and `wraith.json.null` when empty container types or null values matter. integer values must fit the exact JSON-to-Lua number range, from `-9007199254740991` through `9007199254740991`.

## replacement and ownership

provider and tool names use lowercase letters, digits, `_` and `-`, beginning with a letter, up to 64 characters. pass `replace = true` to cover an existing definition. unregistering with `wraith.providers.unregister(name)` or `wraith.tools.unregister(name)` removes the caller's definition and reveals any earlier live owner.

an invocation already bound to a definition keeps that implementation when another definition replaces it. new calls use the accepted replacement. unloading the original owner cancels its existing calls. a rejected unload leaves them running.

## invocation replies and cancellation

| API | behavior |
| --- | --- |
| `wraith.invocation.emit(id, value)` | queue a stream delta |
| `wraith.invocation.finish(id, value)` | queue the terminal result |
| `wraith.invocation.fail(id, message)` | queue a terminal error |
| `wraith.invocation.cancelled(id)` | test whether the call can still accept work |

replies are host effects. they become visible only after earlier effects in that callback succeed. the current callback must belong to the same invocation to emit, finish or fail. returning without a reply leaves the invocation pending until a continuation replies or the deadline expires.

managed resources, scheduled callbacks, dialog callbacks, workspace query callbacks and explicitly nested host callbacks inherit their originating invocation. continuations receive `ctx.invocation.id`; resource callbacks also retain their normal `ctx.resource` and `ctx.data` fields. finishing, failing, cancellation, owner unload or runtime close stops managed children and invalidates later effects. cancellation requests process termination; it does not claim that an operating-system process has already been reaped.

raw `os.execute`, unmanaged subprocesses and arbitrary external side effects are outside this cleanup mechanism. use [managed services](services.md) for owned asynchronous work.

## limits and failures

| item | limit |
| --- | --- |
| hooks | 32 per kind |
| hook payload | 1 MiB, 4096 messages |
| hook callback | 100 ms cooperative execution |
| hook chain | 1 second including queue wait |
| provider names | 32 |
| tool names | 64 |
| replacement layers | 32 per name |
| live invocations | 32 |
| retained invocation handles | 128 |
| queued stream output | 64 chunks and 1 MiB per invocation |
| invocation payload | 1 MiB, depth 32, 16384 values |
| default discovery deadline | 30 seconds |
| default chat deadline | 10 minutes |
| model tool deadline | `execution.command_timeout` |
| Go host invocation deadline | 1 millisecond through 24 hours; defaults to 10 minutes for tools |

the explicit invocation deadline belongs to the Go host API; provider and tool registrations do not expose a Lua timeout option.

stream overflow ends the call with an error and cancels its children. the terminal result remains available after queued deltas. consuming a delta releases its queue charge; closing the host handle releases retained results and any unclaimed start. callback errors fail the invocation and roll back its uncommitted registrations and reservations. effects already accepted by the host are not undone.

cooperative deadlines interrupt Lua bytecode. a blocking native Lua function can still occupy the actor until it returns. callers can cancel and release their waits independently; managed asynchronous services avoid this blocking path.
