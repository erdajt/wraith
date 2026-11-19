# lua services

[configuration and plugins](../plugins.md) explains loading, mappings, commands and trust. these services use the same runtime and package ownership. the [package manager](packages.md) uses them for git processes, private staging files and atomic publication.

## resource lifecycle

asynchronous calls return an opaque resource id. setup reserves work; the host activates it after installing the configuration. callback-created work starts after preceding host actions succeed. rejected setup, rejected registration changes and abandoned action chains discard their reservations.

resource callbacks receive the current workspace snapshot plus:

| field | value |
| --- | --- |
| `ctx.resource.id` | resource handle |
| `ctx.resource.kind` | resource kind or progress event name |
| `ctx.resource.sequence` | increasing event sequence |
| `ctx.resource.revision` | resource revision |
| `ctx.resource.final` | whether this is a terminal event |
| `ctx.data` | detached event data |

terminal data contains `state`, `done`, `result` and optional `error`. successful worker output is under `ctx.data.result`. progress data uses `ctx.data` directly. `done` reports whether the worker has returned; cancellation can be reported before a blocking operating-system call returns.

| API | behavior |
| --- | --- |
| `wraith.resource.status(id)` | inspect retained state, result, error and dropped progress count |
| `wraith.resource.cancel(id)` | request cancellation; repeated cancellation is harmless |
| `wraith.resource.release(id)` | discard the handle after preceding callback effects; cancel outstanding work |

handles belong to the runtime and package that created them. callbacks run under that owner. another package cannot use the handle through these APIs. unload and reload cancel owned resources. completed handles remain queryable until released; release handles you no longer need.

Lua exceptions in a resource callback cancel its remaining work and disable that callback for queued events. `callback_error` remains available in status and terminal data. all Lua callbacks run on the VM owner, never on worker goroutines. terminal events remain deliverable when progress notifications fill their queue. progress may coalesce; a sequence gap does not imply a missing terminal result.

a runtime retains at most 128 handles and admits at most 32 reserved or unfinished workers. cancellation does not free worker capacity until the worker returns. progress queues allow up to 256 events and 8 mib of accounted payload; one event in delivery and terminal results are separate. capture notifications coalesce or report dropped progress; protocol events apply backpressure. a single progress event larger than the byte budget is rejected. retained results and process output have their own per-operation bounds; these are not a total Lua memory limit.

resource callbacks have a 250 ms cooperative Lua deadline. raw native calls can outlast it. event dispatch stays ordered, but nested or deferred host actions may finish after a later event callback. explicit resource updates survive unrelated typing and focus changes; stale implicit draft or current-window edits are discarded. keymap registration conflicts are checked independently.

managed lifecycle is not a sandbox. plugins have the user's account authority through trusted Lua libraries. synchronous filesystem calls run immediately and are not rolled back. cancellation checks prevent queued work from starting, but cannot undo a completed rename or interrupt every kernel call.

## json

```lua
local json = wraith.json
local value = json.decode('{"items":[],"selection":null}')
assert(value.selection == json.null)

local encoded = json.encode({
  items = json.array(),
  metadata = json.object(),
  selection = json.null,
})
```

| API | behavior |
| --- | --- |
| `json.encode(value)` | return a JSON string |
| `json.decode(text)` | return a Lua value, preserving nulls and container kinds |
| `json.null` | explicit null singleton; use it instead of `nil` |
| `json.array(table)` | mark a table as an array; omitted table creates an empty array |
| `json.object(table)` | mark a table as an object; omitted table creates an empty object |

decoded arrays retain null elements. unmarked empty tables encode as objects. nonempty consecutive integer-keyed tables encode as arrays; string-keyed tables encode as objects. constructors mark the supplied table in place.

encoding rejects cyclic, mixed or sparse tables, functions, unsupported userdata, nonfinite numbers and invalid utf-8 strings or keys. decoding rejects invalid utf-8, unmatched surrogate escapes, malformed JSON and trailing values. strings preserve embedded nul, newline and valid Unicode characters. repeated object keys use the last value.

numbers use Lua's floating-point representation. integers beyond its exact range can lose precision; store identifiers and large counters as strings. JSON is text, not a binary transport. filesystem, HTTP and process payloads carry exact byte strings separately.

limits: 1 mib input and encoded output, depth 32 and 16,384 traversal items. object keys count toward the item and string-byte budgets. decoding or encoding errors raise Lua errors and can be caught with `pcall`.

## filesystem

```lua
local fs = wraith.fs
local root = wraith.paths.stdpath("data") .. "/packages"
fs.mkdir(root, { parents = true })
fs.write(root .. "/lock.json", wraith.json.encode({}), {
  overwrite = true,
  mode = 384,
})

fs.read_async(root .. "/lock.json", function(ctx)
  if ctx.data.state == "completed" then
    local lock = wraith.json.decode(ctx.data.result.data)
    wraith.notify("lock loaded")
  else
    wraith.notify(ctx.data.error or ctx.data.state)
  end
  wraith.resource.release(ctx.resource.id)
end)
```

synchronous methods return a result table or raise an error. every method also has an `_async` variant with the same arguments followed by a callback; it returns a resource id. options may be omitted or `nil`. async operation failures arrive in terminal event data.

| method | options | result |
| --- | --- | --- |
| `fs.read(path, opts)` | `offset=0`, `max_bytes=1048576` | `path`, exact `data`, `offset`, `next_offset`, `truncated` |
| `fs.write(path, data, opts)` | `overwrite=false`, `mode=384` | `path`, bytes written |
| `fs.stat(path, opts)` | `follow=false` | `path`, `size`, `mode`, `type`, `modified_ms` |
| `fs.list(path, opts)` | `offset=0`, `limit=1024` | `path`, `entries`, `next_offset`, `truncated` |
| `fs.mkdir(path, opts)` | `parents=false`, `mode=448` | `path` |
| `fs.remove(path, opts)` | `recursive=false` | `path` |
| `fs.rename(source, destination, opts)` | `overwrite=false` | source `path` |

paths resolve against the process working directory when the call is made. paths must be nonempty, contain no nul and fit within 4,096 bytes. `~` and environment substitutions are not expanded. use `paths.stdpath` for application directories.

reads require regular files and follow file symlinks. `max_bytes` accepts 1 through 4,194,304. read offsets are byte offsets, including positions inside a utf-8 sequence. read data preserves every byte. writes accept at most 4 mib.

writes create a private temporary file in the destination directory, write and sync its contents, then install it atomically. `overwrite=false` never replaces an existing entry, including a symlink. `overwrite=true` replaces the destination entry rather than following a destination symlink. failures before installation preserve the old destination and remove the temporary file. replacement uses the requested mode, not the old file's permissions. the directory is not synced, so this API does not promise crash durability.

modes are decimal permission bits: `384` is `0600`, `420` is `0644`, `448` is `0700` and `493` is `0755`. values above `511` are rejected. parent directories created by `mkdir` use the requested mode, subject to the process umask.

listing entries contain `name` and `type`; types are `file`, `directory`, `symlink` or `other`. `limit` accepts 1 through 1,024. each returned page is sorted by name. children removed before their metadata is read are skipped. `next_offset` counts scanned entries, including skipped children. offsets refer to the underlying directory enumeration, not a globally sorted snapshot. directory changes between pages can duplicate or skip entries.

`stat` inspects the symlink itself unless `follow=true`. recursive removal does not follow symlinks to their targets. rename uses an atomic no-replace operation by default; unsupported filesystems return an error. cross-filesystem moves are not copied automatically.

errors include the operation and path. sync calls can block the VM on slow filesystems. async calls check cancellation around reads and before atomic installation; filesystem operations already inside the kernel may finish after cancellation.

## http

```lua
wraith.http.request({
  url = "https://example.com/status",
  method = "GET",
  headers = { Accept = "application/json" },
  timeout_ms = 5000,
  max_bytes = 65536,
}, function(ctx)
  local response = ctx.data.result
  if response and response.status then
    wraith.notify("status " .. response.status)
  end
  if ctx.data.error then
    wraith.notify(ctx.data.error)
  end
  wraith.resource.release(ctx.resource.id)
end)
```

`wraith.http.request(options, callback)` reserves an asynchronous request and returns its resource id.

| option | default and limits |
| --- | --- |
| `url` | required absolute http or https URL; at most 8 kib |
| `method` | `GET`; valid HTTP token, at most 32 bytes |
| `headers` | string-to-string table; at most 128 entries and 64 kib combined |
| `body` | empty exact byte string; at most 4 mib |
| `timeout_ms` | `30000`; integer from 1 through 300000 |
| `max_bytes` | `1048576`; integer from 1 through 4194304 |
| `redirects` | `0`; integer from 0 through 10 |

request headers reject control characters and malformed names. the host header comes from the URL. ordinary TLS certificate verification stays enabled. redirects are disabled by default; when enabled, HTTPS-to-HTTP redirects and non-HTTP schemes are rejected. proxy environment variables follow Go's default HTTP transport.

the result contains `status`, final `url`, `headers`, exact byte `body`, `truncated` and `complete`. header values are arrays so repeated response headers survive. non-2xx statuses are HTTP results, not transport errors.

responses stop at `max_bytes`; `truncated=true` and `complete=false` identify a bounded prefix. transport failures after response headers retain the available status and partial body under `result`, with the cause in `ctx.data.error`. redirect-policy failures may contain headers without a body. explicit cancellation can report immediately before the worker has a partial response to retain. response headers are capped at 64 kib.

## timers

```lua
wraith.timer.start(1000, function(ctx)
  if ctx.resource.final then
    wraith.resource.release(ctx.resource.id)
    return
  end
  wraith.notify("tick " .. ctx.data.tick)
end, { repeat_ms = 1000 })
```

`wraith.timer.start(delay_ms, callback, options)` returns a resource id. omit `repeat_ms` for a one-shot timer. delay and repeat interval must be integer milliseconds from 1 through 86,400,000. zero-delay busy loops are rejected.

a one-shot timer reports one terminal completion, with `tick=1` and `elapsed_ms` under `ctx.data.result`. a repeating timer emits progress events with `ctx.resource.kind="tick"` and `tick`, `elapsed_ms` directly under `ctx.data`. elapsed time starts at activation.

ticks coalesce when the host falls behind. the counter records emitted ticks, so callbacks may observe gaps. repeats schedule the next interval after emitting the current tick; this is not a wall-clock scheduler. cancel through `wraith.resource.cancel(id)`, then release when no longer needed. callback errors cancel repeating timers; unload and reload cancel them as well.

## processes

```lua
local id = wraith.process.start({
  argv = { "git", "status", "--short" },
  cwd = "/path/to/project",
  timeout_ms = 10000,
}, function(ctx)
  if ctx.resource.final then
    local output = wraith.process.output(ctx.resource.id)
    wraith.notify(output.data)
    wraith.resource.release(ctx.resource.id)
  end
end)
```

`process.start(opts, callback)` reserves a process and returns its resource id. this is a trusted plugin API, separate from model command approvals. no terminal input or output is inherited. default stdin is closed.

| option | default | behavior |
| --- | --- | --- |
| `argv` | required unless `shell` is set | consecutive array of 1 through 256 strings; 64 kib total |
| `shell` | unset | explicit shell command; cannot be combined with `argv` |
| `shell_path` | `/bin/sh` | absolute shell executable; requires `shell` |
| `cwd` | application working directory | child working directory; relative paths resolve when reserved |
| `env` | `{}` | string overrides; `false` removes a variable; up to 128 overrides |
| `clear_env` | `false` | start with an empty child environment |
| `stdin` | `false` | keep an input pipe open for `process.write` |
| `timeout_ms` | `0` | no deadline; otherwise 1 through 86,400,000 milliseconds |
| `max_output_bytes` | `262144` | retained tail per stream; 1,024 through 4,194,304 bytes |
| `mode` | `"capture"` | capture notifications or lossless `"protocol"` chunks |

arguments are passed directly to the executable. quoting and shell expansion apply only when `shell` is set. executable names without a path use the application's `PATH`; `env.PATH` changes the child's environment, not executable lookup. use an absolute executable path when overriding lookup matters. environment size is bounded to 1 mib. arguments, paths and environment values reject nul bytes.

`started` arrives before stdout or stderr events and includes `ctx.data.pid`. each stream event includes `stream`, `offset`, `next_offset` and `earliest_offset`. offsets count bytes independently for stdout and stderr; event sequence numbers order notifications across the handle.

capture mode retains a bounded tail and coalesces change notifications. query the bytes you need:

```lua
local page = wraith.process.output(id, {
  stream = "stderr",
  offset = 0,
  max_bytes = 65536,
})
```

`stream` defaults to `stdout`. `offset` defaults to zero. `max_bytes` defaults to 64 kib and accepts 1 through 1,048,576. results contain exact `data`, actual `offset`, `next_offset`, `earliest_offset`, `observed_bytes`, `truncated` and `more`. `truncated=true` means the requested prefix was already evicted. querying a completed process remains valid until release. cancellation before activation can finish without creating output streams; `process.output` then reports that the process has not started. use terminal status to distinguish this case from a running process with an empty output page.

protocol mode includes exact `ctx.data.data` bytes in each stream event. it preserves nul, invalid utf-8, partial Unicode sequences and framing bytes. operating-system reads can split or combine protocol frames. chunks are at most 32 kib. backpressure pauses producers when the event queue fills; it does not silently drop protocol bytes. parse protocol state in the event callback before deferring UI actions. callbacks dispatch in event order, but a nested UI action or editor can finish after a later event callback.

```lua
wraith.process.start({
  argv = { "/path/to/language-server", "--stdio" },
  stdin = true,
  mode = "protocol",
}, function(ctx)
  if ctx.resource.kind == "started" then
    local body = wraith.json.encode({
      jsonrpc = "2.0", id = 1, method = "initialize", params = {},
    })
    wraith.process.write(ctx.resource.id,
      "Content-Length: " .. #body .. "\r\n\r\n" .. body)
  elseif ctx.resource.kind == "stdout" then
    -- append bytes to your protocol parser here.
  end
end)
```

`process.write(id, bytes)` queues bytes and returns the accepted byte count. it does not wait for the child to read them. each write accepts at most 64 kib; pending input is bounded to 1 mib and 32 requests. full queues raise an error so the caller can retry later. `process.close_stdin(id)` queues EOF after earlier writes and rejects subsequent writes. both require a started process with `stdin=true`.

pipe failures produce a `stdin` event with `written` and `error`; terminal results also include `stdin_error` when present. a process's terminal result contains `pid` and `exit_code`; launch, timeout and nonzero-exit errors appear in terminal `error`. inspect lifecycle with `resource.status`, stop with `resource.cancel`, and discard with `resource.release`.

cancellation kills the process group and reaps the direct child. ordinary descendants are also stopped when the parent exits. descendants that create a separate session or process group can escape this cleanup. cancellation notification can precede reaping; `resource.status(id).done` distinguishes them. managed processes cannot detach from their owner.
