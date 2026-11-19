# architecture

wraith is a single go binary. bubble tea owns terminal lifecycle and the event loop; the app renders its own themed cells. ollama inference and pty reads run outside the event loop and return messages to it. application state is mutated by the event loop, while workers capture request settings and session snapshots.

| package | responsibility |
| --- | --- |
| `cmd/wraith` | flags, diagnostics, workspace lock, session selection and terminal startup |
| `internal/config` | defaults, validation, xdg locations and configuration persistence |
| `internal/keymap` | vim key notation, modal sequences and action resolution |
| `internal/plugin` | trusted Lua VM ownership, explicit module paths, configuration and callback effects |
| `internal/provider` | normalized api and cli backends, streaming events and model catalogs |
| `internal/runner` | concurrent approved jobs, process groups, recovery journals and redacted output queries |
| `internal/variables` | annotated scoped bindings, immutable redaction snapshots and parser-backed shell substitution |
| `internal/modeltool` | shared tool schemas, strict arguments and provider-neutral definitions |
| `internal/markdown` | themed markdown blocks, inline formatting and responsive tables |
| `internal/ollama` | native model discovery, capability inspection and bounded streaming requests |
| `internal/session` | versioned chat records, message metadata and isolated copies |
| `internal/contextual` | request assembly, token budgets, extraction, auditing and compaction policy |
| `internal/store` | writer lock, gzip sessions, immutable archives and decompression limits |
| `internal/fsutil` | private temporary files and synced atomic replacement |
| `internal/editor` | external editor lifecycle, save detection, fenced blocks and quoted staging |
| `internal/terminal` | linux ptys, terminal key encoding, vt parsing, screen cells and scrollback |
| `internal/vim` | rune-oriented prompt buffer, modal operations, registers, search and undo |
| `internal/ui` | views, input routing, commands, menus and workflow transactions |

## requests and cancellation

ollama uses `/api/chat`, `/api/tags`, `/api/show` and an empty `/api/generate` preload. cloud adapters normalize server-sent events; CLI adapters consume JSONL from isolated child processes. requests have context deadlines. streamed content and reasoning are accumulated separately. an incomplete stream is an error, and received text is saved with an interrupted marker. requests use native thinking values, not synthetic reasoning instructions.

agent mode distinguishes execution requests from ordinary code suggestions. a validated request stops at approval. only an explicit run action creates a tracked process. agent jobs return bounded evidence unless hidden; blocks-mode output requires sharing or auto_share. immutable launch records freeze approvals. the runner manager coalesces updates, and only the event loop appends durable results to the originating session. CLI providers act as text backends with native tools disabled; wraith owns the execution flow. see [execution](execution.md) and [providers](providers.md).

`/models` retains the transcript while selecting a new model and probing its capabilities. selections arriving during an active generation are dismissed. shell reads identify their originating pty, so events from a replaced shell cannot update its replacement.

## persistence

writes use a temporary file in the destination directory, file synchronization, rename and directory synchronization. files are private. session ids and archive names are validated before path construction. gzip reads are bounded and checked for corruption. an exclusive data-directory lock prevents concurrent interactive writers. sessions record their canonical launch directory and provider; resume checks the directory before activating a session. input history is compressed separately and keyed by directory.

compaction first creates recovery copies, then asks the model for a replacement. only a successfully persisted replacement becomes active. response editing follows the same archive-before-replace pattern. see [context.md](context.md) for the full transaction and preservation limits.

## rendering and input

model text passes through control-sequence filtering before display. shell output goes through the vt parser instead of being forwarded directly to the host terminal. clipboard escape strings and terminal control payloads from a model cannot become host-terminal commands through the transcript renderer.

ordinary terminal text, including dedicated space events and bracketed paste, is forwarded to the selected pty. the centered pane clears covered cells to the default host background when inheritance is selected. a separate screen and scrollback belong to each pty. triple escape snapshots these cells into a read-only vim buffer, keeping selections stable while the shell continues. normal shell exit schedules a replacement with a rapid-failure restart limit.

the renderer is intentionally text-only. sixel/kitty graphics, arbitrary terminal extensions, full vim emulation, font selection and independent pane opacity are not implemented. platform support currently targets linux amd64 and arm64; only the native architecture's tests execute locally.

## private data and output

all provider requests pass a shared privacy boundary, including CLI discovery, compaction and corrective retries. typed evidence keeps structural ids and schemas separate from redacted values. compaction retains known run-reference provenance, and output tools enforce session ownership and the approved sharing policy. a model request cannot grant itself access to hidden output.

private snapshots are immutable for a request or approval. variable writes use a revision check and locked retained-value journal so rotation does not forget historical literals. parser-backed shell substitution produces literal shell words; non-shell source requires editor review. streamed invalid references cancel the attempt, show a retry status and replace superseded text. discarded attempts are compressed separately from accepted chat history.

live job journals hold an owner lock, retained prefix, independent tail and checkpoints. final persistence precedes automatic completion delivery; session delivery ids make evidence insertion idempotent. output tools pause for compaction when evidence cannot fit, without consuming their cursor. readers freeze during selection so incoming output cannot change a yank.

## lua ownership

one worker owns each Lua VM. setup produces a candidate manifest with validated settings, keymaps, commands and callbacks. the UI validates that manifest against its action catalog before installing it. failed reloads leave the current runtime untouched.

callbacks receive copied snapshots and return bounded effects. implicit workspace effects check runtime identity, input ownership, selected-message state, draft and settings before application. explicit surface and resource effects instead check their owner, revision and lease, so unrelated typing does not discard background work. queued workspace actions use the same guards as mapped actions; an approval decision is never exposed as a plugin action. trusted Lua can execute processes directly outside those guards. cancellation releases callers immediately; blocking native calls can still delay worker shutdown. editor continuations resume after the editor result has been applied.

runtime commands, mappings and subscriptions use a publication handshake. the VM stages its registry, returns a detached manifest and waits for the UI to validate and acknowledge it. the acknowledgement uses a separate channel from callback requests. a nested callback therefore sees the installed registry without blocking behind its parent's publication. rejection restores the prior registry before cleanup runs; acknowledgement never waits for arbitrary cleanup code.

registration layers retain their owner. a replacement covers earlier definitions, and a deletion covers custom definitions while allowing built-in fallback. removing an owner exposes the surviving layer. explicit configuration changes become configuration-owned layers, including deletions, so later plugin removal cannot revive a mapping the user removed. callback references remain alive while covered definitions still need them.

plugin setup has its own registry transaction inside the surrounding callback. a caught setup failure releases newly registered cleanup hooks and restores that setup's partial registrations. callback rejection releases its newly owned resources too. neither transaction reverses Lua globals, cached modules or external side effects. cleanup diagnostics have a separate bounded channel and remain visible after reload, labelled with their previous configuration origin.

window and session state events may coalesce to the newest queued snapshot. completion and custom events remain ordered across registry publications. event payloads cross the VM boundary as detached values. rendering never invokes Lua.

YAML and effective configuration remain separate. settings persistence applies only explicit changes to YAML, preserving Lua-only overrides and credentials. see [plugin lifecycle and limits](plugins.md#runtime-boundaries).


## extension services and interfaces

managed processes, http requests and timers run outside the VM worker. completion enters its ordered queue, then uses the same host effect pipeline as other callbacks. structural registry changes require publication acknowledgement. handles belong to the registering plugin; unload cancels owned work. completed handles remain available until released. callbacks, retained results and queues have [explicit bounds](lua/services.md). direct `os` and `io` calls bypass these facilities and run with the user's authority.

plugin buffers and windows use native rendering and input routing. a buffer owns text and revisions; windows own their cursor, selection, scroll and search. multiple windows can share one buffer. updates carry revision and lease checks so a delayed callback cannot replace a newer edit or a reused window. plugin regions contribute text and styles to named presentation positions. dialogs carry detached choices and return a selected result to the owning callback. no renderer calls into lua. see [surface contracts](lua/surfaces.md).

workspace queries use snapshots and bounded pages. session catalogs and saved output reads run in workers; cancellation, runtime identity and session ownership are checked before delivery. local output queries can read hidden local output but do not change native sharing flags. these APIs provide navigation data, not permission to attach private output to a native model request. see [workspace queries](lua/workspace.md).

request hooks run before the final privacy and evidence checks. command hooks cannot bypass approval or grant sharing. custom providers use the normalized streaming path; custom tools validate their schemas and arguments before native approval. protected evidence is carried separately from plugin-authored text. trusted code can still make independent network calls, so these checks protect the native workflow, not against a malicious plugin. see [extension boundaries](lua/extensions.md).

## packages

`internal/plugin/lua/wraith/manager` is bundled lua loaded through ordinary `require`. it uses public lifecycle, filesystem, process, timer and surface APIs. it has no private installer privilege. configuration chooses when to call `setup`; startup never installs or updates sources.

spec normalization rejects unknown fields, conflicting source identities, duplicate names and dependency cycles before running initialization. each package has separate initialization and activation owners. lazy commands, events and keys preserve the first triggering input. dependencies activate first, and owned cleanup runs during unload. an exposed module's identity remains fixed for that VM even after unload: changing its source, path, main module or installed revision requires a fresh runtime. this prevents a new lockfile from mislabelling an old `require` cache.

git operations prepare unique staging directories and verify immutable objects by source, full revision and checkout cleanliness. publication holds a persistent `flock` inode, compares the captured lockfile baseline and atomically replaces it. an existing object is verified, never reset or cleaned. local directories are not managed checkouts. git ignores ambient global, system and working-directory configuration; credentials and transport overrides follow the documented allowlist.

cancellation after publication checks the actual generation before reporting an outcome. success removes only the transaction's own staging and baseline files; failed cleanup remains visible without turning an installed revision into a failed install. rejected UI publication retains cleanup receipts until canonical handles disappear. uncertain or failed transactions preserve evidence for bounded, read-only recovery inspection. the lock protocol does not promise power-loss durability or automatic garbage collection. see [package lifecycle, locking and recovery](lua/packages.md).

## bundled lua runtime

[the local gopher-lua replacement](../third_party/README.md) retains upstream source, tests and license. its protected-call patch closes only unwound stack frames and preserves live caller upvalues after caught errors. generator sources and generated files carry the same changes. the upstream suite runs separately because `go test ./...` excludes nested modules.
