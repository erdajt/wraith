# architecture

wraith is a single go binary. bubble tea owns terminal lifecycle and the event loop; the app renders its own themed cells. ollama inference and pty reads run outside the event loop and return messages to it. application state is mutated by the event loop, while workers capture request settings and session snapshots.

| package | responsibility |
| --- | --- |
| `cmd/wraith` | flags, diagnostics, workspace lock, session selection and terminal startup |
| `internal/config` | defaults, validation, xdg locations and configuration persistence |
| `internal/provider` | normalized api and cli backends, streaming events and model catalogs |
| `internal/runner` | approved jobs, process-group cancellation and compressed output history |
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

Ollama uses `/api/chat`, `/api/tags`, `/api/show` and an empty `/api/generate` preload. cloud adapters normalize server-sent events; CLI adapters consume JSONL from isolated child processes. requests have context deadlines. streamed content and reasoning are accumulated separately. an incomplete stream is an error, and received text is saved with an interrupted marker. requests use native thinking values, not synthetic reasoning instructions.

agent mode distinguishes execution requests from ordinary code suggestions. a validated request stops at approval. only an explicit run action creates a tracked process. command output stays outside model context until manually shared, unless automatic sharing is enabled. CLI providers act as text backends with native tools disabled; Wraith owns the execution flow. see [execution](execution.md) and [providers](providers.md).

`/models` retains the transcript while selecting a new model and probing its capabilities. selections arriving during an active generation are dismissed. shell reads identify their originating pty, so events from a replaced shell cannot update its replacement.

## persistence

writes use a temporary file in the destination directory, file synchronization, rename and directory synchronization. files are private. session ids and archive names are validated before path construction. gzip reads are bounded and checked for corruption. an exclusive data-directory lock prevents concurrent interactive writers. sessions record their canonical launch directory and provider; resume checks the directory before activating a session. input history is compressed separately and keyed by directory.

compaction first creates recovery copies, then asks the model for a replacement. only a successfully persisted replacement becomes active. response editing follows the same archive-before-replace pattern. see [context.md](context.md) for the full transaction and preservation limits.

## rendering and input

model text passes through control-sequence filtering before display. shell output goes through the vt parser instead of being forwarded directly to the host terminal. clipboard escape strings and terminal control payloads from a model cannot become host-terminal commands through the transcript renderer.

ordinary terminal text, including dedicated space events and bracketed paste, is forwarded to the selected pty. the centered pane clears covered cells to the default host background when inheritance is selected. a separate screen and scrollback belong to each pty. triple escape snapshots these cells into a read-only Vim buffer, keeping selections stable while the shell continues. normal shell exit schedules a replacement with a rapid-failure restart limit.

the renderer is intentionally text-only. sixel/kitty graphics, arbitrary terminal extensions, full vim emulation, font selection and independent pane opacity are not implemented. platform support currently targets linux amd64 and arm64; only the native architecture's tests execute locally.
