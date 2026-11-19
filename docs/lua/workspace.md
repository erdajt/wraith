# workspace services

[Lua configuration](../plugins.md) · [buffers and views](surfaces.md) · [managed services](services.md) · [example packages](examples.md)

workspace queries read native conversations, commands and metadata without blocking the interface. callbacks run on the Lua worker with a fresh workspace snapshot. these APIs are for trusted local plugins. reading command output does not share it with the model.

## queries

```lua
local query = wraith.jobs.list({ limit = 40 }, function(ctx)
  if not ctx.data.ok then
    wraith.notify(ctx.data.error.message)
    return
  end
  for _, job in ipairs(ctx.data.result.page.items) do
    wraith.notify(job.status .. ": " .. job.command)
  end
end)

wraith.workspace.cancel(query)
```

all methods take an options table and callback, then return an opaque query ID. invalid options raise a Lua error before work starts. setup-time queries wait for `ready`; callback-time queries start after their effect reaches the host.

| method | options | result |
| --- | --- | --- |
| `sessions.list` | `directory`, `cursor`, `limit` | paged session metadata and current session |
| `sessions.get` | `id`, `directory` | one session's metadata |
| `sessions.messages` | `id`, `revision`, `from`, `limit` | message summaries |
| `sessions.message` | `id`, `revision`, `index`, `field`, `from`, `limit` | one message text window |
| `jobs.list` | `session`, `directory`, `cursor`, `limit` | paged command metadata and live jobs |
| `jobs.get` | `id` | one command's metadata |
| `jobs.output` | `id`, `revision`, `from`, `pattern`, `regex`, `ignore_case`, `before`, `after`, `limit` | bounded output page and command metadata |
| `environment.list` | none | variable names, redacted comments and scope |
| `terminals.list` | none | scratch terminal identities, dimensions and status |

prefix each method with `wraith.`. pass `{}` when no options are needed. unknown fields are rejected. session and command IDs use the native 24-character lowercase hexadecimal format. omission of a session ID selects the current conversation; command IDs are required for direct command reads.

callbacks receive either:

```lua
ctx.data = { id = query_id, kind = "jobs.list", ok = true, result = result }
ctx.data = { id = query_id, kind = "jobs.list", ok = false,
  error = { code = "stale_revision", message = "..." } }
```

error codes include `cancelled`, `deadline`, `not_found`, `wrong_directory`, `stale_revision`, `limit` and `io`. inspect the message for the failed record or bound. unreadable catalog records appear in `page.errors`; they do not silently disappear or stop later pages.

## catalog pages

`sessions.list` defaults to the current directory. `jobs.list` defaults to the current session, including its commands executed in other directories. explicit `session` and `directory` filters are mutually exclusive. an explicit directory selects commands by their execution directory. comparisons resolve directory aliases when possible.

```lua
{
  page = {
    items = {}, errors = {}, next = "opaque cursor", complete = false, examined = 4,
  },
  active = {},
}
```

`active` contains captured live records on the first page only. merge by `id`, preferring live records over persisted records. catalog entries are sorted within each page by timestamp. pages follow filesystem enumeration, not a globally newest-first index. a plugin can sort its loaded records after merging.

`limit` defaults to 40 and allows 1 through 100; each page examines at most four directory entries. filtered pages can contain no items and still have `next`. continue until `complete`, not until the first empty page.

continuation tokens are single-use and bind the runtime, package owner, provider invocation, filter and directory generation. they expire after two idle minutes. directory changes invalidate an in-progress scan; restart without a cursor. no snapshot consistency is promised while another process changes history.

`workspace.cancel(query_id)` cancels a pending query and closes its unconsumed continuation. after a cursor advances, ownership moves to the next query ID. cancelling an older page cannot close the newer page's iterator. use this when a view stops loading early. foreign owners cannot cancel each other's queries. unloading the owner or closing its provider invocation also closes its queries and iterators.

## conversations

session metadata contains `id`, `title`, `directory`, `model`, `provider`, `created`, `updated`, `message_count`, `draft_bytes` and `revision`. the revision identifies transcript content, roles, edits and interruption flags. it does not track unrelated title or draft changes.

`sessions.messages` uses a zero-based message index in `from`, with 40 entries by default and 100 maximum. its result contains `items`, `revision`, `next` and `complete`. each item has `index`, `id`, `role`, `preview`, `content_bytes`, `thinking_bytes`, `edited` and `interrupted`. previews are at most 1 KiB. append-only updates preserve existing message identities; edits and compaction can replace them.

`sessions.message` requires the expected `revision` and a zero-based `index`. `field` is `content` by default or `thinking`. `from` is a UTF-8 byte offset; `limit` defaults to 16 KiB and allows up to 64 KiB. its result is `{ revision, text, next, complete }`. offsets must fall on UTF-8 boundaries. chunks end on complete runes. a limit too small for the next rune returns an error.

stored session reads enforce the active directory. `sessions.get` can explicitly select another directory. native resume behavior and its directory checks are unchanged by these read APIs.

## commands and output

command metadata includes `id`, `session`, `directory`, `command`, `shell`, `source`, `started`, `finished`, `status`, `exit_code`, `error`, `background`, `agent`, `share_output`, `output_hidden`, `observed_bytes`, `captured_bytes`, `truncated` and `revision`. metadata excludes captured output and private environment assignments.

`jobs.get` and `jobs.output` resolve live jobs before saved records. commands belonging to the current session remain readable when their execution directory differs. other-session commands must match the active directory.

```lua
wraith.jobs.output({ id = command_id, pattern = "error", ignore_case = true,
  before = 2, after = 2, limit = 40 }, function(ctx)
  if not ctx.data.ok then return end
  local result = ctx.data.result
  for _, line in ipairs(result.page.lines) do
    wraith.notify(tostring(line.line) .. ": " .. line.text)
  end
end)
```

output pages use native line numbers. `from = 0` starts from the beginning; `next` continues the window. patterns match literals by default or bounded RE2 expressions with `regex = true`. `before` and `after` allow 0 through 10 lines; `limit` defaults to 80 and allows up to 200. returned text is capped at 16 KiB. the page reports matching, retained and observed output statistics, truncation and continuation. captured prefix/tail gaps remain gaps; a plugin cannot retrieve discarded bytes.

`revision` is optional for output reads. provide a previously received command revision to reject a changed status or output extent. trusted local reads may inspect hidden output, but never change sharing flags, insert model evidence or resume an agent. [native output sharing](../execution.md) remains separate.

## environment and terminals

`environment.list` returns `{ items, directory, session, scope }`. items contain variable names and comments with configured values redacted. values and process environment assignments are omitted. `scope` describes the configured retention policy.

`terminals.list` returns `{ items }`. each item contains `id`, `index`, `generation`, `visible`, `running`, `width`, `height` and `error`. restarting a terminal changes its identity. this method does not expose a PTY writer or copy terminal contents into a model request.

## lifecycle events

```lua
wraith.events.on("jobs_changed", function(ctx)
  wraith.notify("job state changed: " .. tostring(ctx.data.sequence))
end)
```

`jobs_changed`, `terminals_changed` and `environment_changed` provide bounded resynchronization data in `ctx.data`: `sequence`, `items`, `session`, `directory` and `resync = true`. environment changes include `scope`; job changes include removed IDs when known. events omit command output and variable values.

updates can coalesce while callbacks are busy. they are not a lossless audit log. event order describes accepted host observations, not every process write. use saved command records for durable history, and query current state after an event. rendering does not wait for these callbacks.

## bounds and failure handling

| bound | value |
| --- | --- |
| pending queries, including replies waiting for Lua | 16 per runtime |
| retained scan iterators | 16 per runtime |
| query work deadline | 10 seconds |
| concurrent source readers through result projection | 2 |
| decoded direct source | 64 MiB per record |
| decoded catalog source | 16 MiB per record |
| catalog entries examined per page | 4 |
| detached callback result | 256 KiB |

source budgets bound concurrent decoded input to 128 MiB, excluding JSON allocations, intermediate copies and captured live state. they are not a process memory limit. command metadata sidecars avoid decoding large captures for catalog reads. missing or stale sidecars fall back to bounded payload reads. old oversized records return a limit error with page continuation; direct output reads have a separate 64 MiB source limit.

callbacks resolve once while their owner is alive. explicit cancellation delivers a cancelled result when the callback remains available; unload, reload and provider cancellation discard dead callbacks. queue pressure retains bounded replies until Lua can accept them. filesystem operations that block inside the kernel can delay cleanup despite the cooperative deadline.
