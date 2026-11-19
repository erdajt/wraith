# buffers, windows and presentation

extensions can create shared text buffers, tiled or floating views, typed presentation regions and modal pickers. the host owns rendering and input. Lua callbacks publish text and styles; they never run from the render loop.

[configuration and lifecycle](../plugins.md) · [native input mappings](../keybindings.md) · [managed services](services.md)

## open a workspace buffer

```lua
wraith.commands.create("notes", function()
  local buffer = wraith.buffers.create({
    name = "review notes",
    editable = true,
    lines = { "scope", "", "findings" },
  })
  wraith.buffers.set_keymap(buffer, "buffer.normal", "<Space>f", { keys = "G" })
  wraith.windows.open(buffer, { position = "right", width = 48, focus = true })
end, "open review notes")
```

open another window with the same buffer id to share text and undo history. cursor, scroll, search, mode and selection belong to each window. default Vim splits preserve that relationship. closing a window keeps its buffer; deleting a buffer closes every view of it.

read-only buffers accept motions, search, visual selection and yank. editable buffers also accept insertion, deletion, replacement and undo. buffer mappings inherit conversation mappings outside insert mode. local mappings take precedence. mouse clicks focus the topmost view and place its rune cursor when mouse support is enabled.

## buffers

| API | result |
| --- | --- |
| `buffers.create(options)` | opaque buffer id; options: `name`, `editable`, `lines`, `spans` |
| `buffers.list()` | metadata: id, owner, name, revision, text revision, line and byte counts |
| `buffers.get(id)` | detached text, styles, effective mappings and metadata |
| `buffers.get_lines(id, start, stop)` | bounded page with `lines`, `start`, `next` and metadata |
| `buffers.set_lines(id, start, stop, lines, revision)` | replace a line range; return new revision |
| `buffers.set_spans(id, spans, revision)` | replace the caller's style layer; return new revision |
| `buffers.delete(id, revision)` | delete the buffer and its views |
| `buffers.native("prompt")` | native prompt projection id, or `nil` before activation |
| `buffers.native("conversation")` | native transcript projection id, or `nil` before activation |

`get(id)` returns the whole stored buffer; use `get_lines` when bounded reads matter. setters also apply the shared Lua argument limits: 32 levels, 16,384 values and 1 mib of string data per argument. build larger buffers through bounded incremental edits.

all names above are under `wraith`. indexes are zero-based. line ranges are half-open: `[start, stop)`. use `-1` for the current end. an empty replacement deletes that range. a buffer always retains at least one line. `get_lines` returns at most 1 mib of complete lines; continue at `next` until reaching the requested end or `line_count`.

`revision` is optional. omitting it uses the revision visible to that callback. supply a saved revision when applying a result computed from earlier text. a mismatch rejects the edit instead of replacing newer work. text, styles and local mappings advance the buffer revision; `text_revision` advances only when text changes.

user edits update the canonical store and all visible views. cursor positions after an external edit rebase through the changed range. insertions at an anchor leave that anchor before the inserted text; deleted anchors clamp to the replacement start. overlapping visual selections collapse. external text replacements reset incompatible undo history; unchanged text and style updates retain it.

### local mappings

```lua
wraith.buffers.set_keymap(buffer, "buffer.normal", "r", function()
  wraith.notify("refresh requested")
end, "refresh this view")
wraith.buffers.set_keymap(buffer, "buffer.normal", "x", { keys = "dd" })
wraith.buffers.del_keymap(buffer, "buffer.normal", "r")
```

`set_keymap(id, mode, keys, target, description, revision)` accepts an action name, callback or `{keys = "native sequence"}`. `del_keymap(id, mode, keys, revision)` hides local overrides for that sequence. use `noop` to consume a built-in binding. the actual native prompt and transcript use `prompt.*` and `chat.*` modes. generic windows use `buffer.*` modes, including windows opened on a native projection.

mapping and decoration layers belong to the caller, even when another package created the buffer. unloading the caller reveals earlier live layers. unloading a referenced callback owner removes mappings targeting it. text edits clear decorations because their old ranges no longer describe the same text.

### native projections

read native ids from a `ready` callback or later. the prompt projection supports revision-checked text edits. the conversation projection is read-only; it contains role labels and message text rather than the styled transcript. opening either projection in another window uses the generic buffer renderer.

native projections are bounded to the latest 1 mib, 100,000 lines and 64 kib per line. unsupported controls are removed. `truncated = true` means the projection is incomplete or lossy; an incomplete prompt cannot be edited through this API. native projections cannot be deleted. presentation regions can replace native prompt and welcome rendering without replacing their input ownership.

## styles and text

```lua
wraith.buffers.set_spans(buffer, {
  {
    start = { line = 0, column = 0 },
    ["end"] = { line = 0, column = 5 },
    style = { foreground = "accent", bold = true },
  },
})
```

columns count Unicode runes, not bytes or terminal cells. spans are half-open and must stay within existing text. rendering expands partial combining or joined grapheme ranges to whole display cells. later overlapping spans take precedence. tabs occupy four-column stops while the canonical text retains its tab rune.

styles support `foreground`, `background`, `bold`, `italic` and `underline`. colors accept `#rrggbb`, an empty inherited value, or these theme keys:

`bg`, `fg`, `muted`, `accent`, `second`, `border`, `warn`, `variable`, `hint_quit`, `hint_refresh`, `hint_cancel`.

buffer lines must contain valid UTF-8. newlines separate lines; tabs are allowed within them. raw terminal escape sequences and other controls are rejected. managed process and filesystem APIs preserve bytes. call `wraith.ui.decode(bytes)` explicitly before displaying binary output; it replaces invalid UTF-8 and removes controls except tabs and newlines. it does not interpret terminal escape sequences as styling.

## windows

| API | behavior |
| --- | --- |
| `windows.open(buffer, options)` | reserve an opaque owned view id |
| `windows.get(id)` | detached owned view configuration and cursor |
| `windows.configure(id, options, revision)` | merge options into an owned view |
| `windows.focus(id, revision)` | focus an owned view |
| `windows.close(id, revision)` | close an owned view |
| `windows.list()` | native snapshot views and owned views |

owned window ids are strings. existing native conversation ids are numbers. `windows.split("horizontal" or "vertical")`, numeric `windows.focus(id)` and numeric or omitted `windows.close(id)` retain their native behavior. use an opaque id with `get` and `configure`.

window options:

| field | meaning |
| --- | --- |
| `kind` | `split` or `float`; default `split` |
| `position` | `right`, `left`, `above`, `below`; default `right` |
| `relative_to` | another live tiled owned window; omitted means current tiled view |
| `width`, `height` | cell dimensions; zero gives automatic split sizing; floats require positive values |
| `row`, `column` | nonnegative floating offsets within the conversation body |
| `title` | frame label; otherwise use the buffer name |
| `focus` | request focus for this operation |
| `cursor` | `{line = 0, column = 0}` in rune coordinates |
| `mode` | `normal` or `insert`; insertion requires an editable buffer |
| `buffer_id` | change the displayed buffer with `configure` |

changing kind or placement moves the existing view. requested split sizes adjust the nearest matching divider and respect minimum visible sizes. terminal resize clips floats and may hide split branches until space returns. user divider changes update owned window geometry. `focus` is an operation request, not persistent focus state. window `mode` records normal versus insertion cursor behavior; visual selection and search remain transient state in each view.

native approvals, editors and modal dialogs retain input ownership. focus requests that would replace them fail. passive regions never capture input.

## presentation regions

```lua
local header = wraith.regions.create({
  slot = "header",
  lines = { "review workspace" },
})
wraith.regions.set(header, { lines = { "review workspace · ready" } })
```

`regions.create(options)` returns an id. `regions.get(id)` returns its frame. `regions.list()` returns metadata. `regions.set(id, options, revision)` merges frame fields; `regions.delete(id, revision)` removes it.

| slot | placement |
| --- | --- |
| `header` | first application row; one line |
| `footer` | key-hint row; one line; active command and search entry remain visible |
| `sidebar` | right edge of the conversation body; `width` from 1 to 120 |
| `body` | passive overlay with positive `width` and `height`, optional `row` and `column` |
| `welcome` | centered within empty native conversation views; optional `width` |
| `prompt` | four presentation lines inside the native prompt frame; input stays native |

frames use `lines` and the same typed `spans` as buffers. the newest live region wins a replacement slot; removing it reveals the previous owner. body overlays compose in creation order. sidebar width reduces conversation space without shrinking the prompt or native side shell. frames clip to available space and remain below dialogs and terminal overlays.

## select and input

```lua
wraith.ui.select({
  { id = "ports", label = "ports", detail = "review service exposure" },
  { id = "http", label = "http", detail = "review application behavior" },
}, { title = "work area" }, function(ctx)
  if not ctx.data.cancelled then
    wraith.notify(ctx.data.item_id)
  end
end)

wraith.ui.input({ title = "review note", initial = "" }, function(ctx)
  if not ctx.data.cancelled then
    wraith.notify(ctx.data.value)
  end
end)
```

select items can also be strings. explicit ids must be unique. `j/k` or arrows move; `/` filters labels and details; Enter accepts the first matching item without an extra confirmation. the callback receives `value`, `item_id` and the original one-based `index` in `ctx.data`.

input starts in insert mode. Escape enters normal mode, then cancels when pressed again. Enter or Ctrl+S accepts. native Vim editing, search, selection and yank use the buffer input scopes. picker native-input mappings use `picker`. dialog callbacks receive `cancelled = true` on cancellation, optionally with a `reason` such as `busy`.

requests resolve once. callbacks run only while their runtime and owner remain alive. unloading or replacing the owner closes its dialog and discards its callback. opening a dialog while another modal owns input returns a cancelled result instead of stealing focus. ordinary workspace actions cannot submit or alter the hidden conversation draft while a dialog owns input.

## lifecycle, errors and limits

setup can create buffers, windows and regions. creation reserves ids immediately but becomes visible only after configuration and host layout validation succeed. setup dialogs are scheduled after activation. explicit native projections become available at `ready`.

within a callback, reads include its own staged changes. errors discard uncommitted changes and reservations. after a successful callback, the host applies effects in order. each mutation checks runtime ownership and its expected revision. an earlier effect that has already committed is not rolled back if a later host effect fails. unrelated typing and cursor movement do not invalidate explicit surface edits; stale target revisions still do.

objects belong to their creating package instance. unloading it closes its windows, buffers, regions and dialogs and restores a live native view when needed. trusted packages can read or edit another package's explicitly supplied ids. these are lifetime guards, not a security sandbox.

| resource | limit |
| --- | --- |
| owned buffers | 64; 4 mib each; 16 mib total |
| buffer lines | 100,000; 64 kib per line |
| owned windows | 64, also subject to the workspace window cap |
| regions | 32; 256 kib and 200 lines per frame; narrower slot limits apply |
| select items | 1,024; 256 kib total; label and detail each 4 kib |
| input text | 64 kib |
| pending callbacks | 128 shared with scheduled callbacks |

release unused objects explicitly. surface storage does not consume managed process, HTTP or timer worker slots.

## text change events

`wraith.events.on("buffer_changed", callback)` receives `ctx.data.buffers`, the
complete current metadata index. Compare `id` and `text_revision` with your previous
index. Missing ids were removed. This is a resynchronization event, not an edit log;
several edits may become one delivery. Styling and mappings do not trigger text events.
The host retains the latest index when the callback queue is full and retries on the
next update. There is no source attribution or promise to deliver each intermediate edit.

`ctx.surfaces` contains the current buffer, window and region metadata indexes.
The compatibility `ctx.prompt` mirror is limited to 32 kib, with
`ctx.prompt_truncated` marking an incomplete prefix. `ctx.argument` is limited to
8 kib with `ctx.argument_truncated`. Recent message content shares a 64 kib budget.
Native buffers provide paged access to their bounded projections. An explicit prompt
replacement over 1 mib is rejected before the actual draft changes.

Startup registration accepts at most 128 surface mutations and 8 mib of encoded
mutation data in total, including metadata and repeated replacements. Accepted
startup records and their preview are released after installation, including when
the plugin registers no callbacks. Live buffers and windows remain available.
