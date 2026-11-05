# architecture

wraith is a single go binary. bubble tea owns terminal lifecycle and the event loop; the app renders its own themed cells. ollama inference and pty reads run outside the event loop and return messages to it. application state is mutated by the event loop, while workers capture request settings and session snapshots.

| package | responsibility |
| --- | --- |
| `cmd/wraith` | flags, diagnostics, workspace lock, session selection and terminal startup |
| `internal/config` | defaults, validation, xdg locations and configuration persistence |
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

chat requests use `/api/chat`; installed models use `/api/tags`; model capabilities use `/api/show`. requests have context deadlines. streamed content and reasoning are accumulated separately. an incomplete stream is an error, and received text is saved with an interrupted marker. requests use native thinking values, not synthetic reasoning instructions.

there is no tool-calling or autonomous shell execution loop. the model proposes text. explicit user input opens editors, stages snippets and sends terminal input. shell output is not automatically injected into model context; paste relevant evidence into a prompt.

`/models` retains the transcript while selecting a new model and probing its capabilities. selections arriving during an active generation are dismissed. shell reads identify their originating pty, so events from a replaced shell cannot update its replacement.

## persistence

writes use a temporary file in the destination directory, file synchronization, rename and directory synchronization. files are private. session ids and archive names are validated before path construction. gzip reads are bounded and checked for corruption. an exclusive data-directory lock prevents concurrent interactive writers.

compaction first creates recovery copies, then asks the model for a replacement. only a successfully persisted replacement becomes active. response editing follows the same archive-before-replace pattern. see [context.md](context.md) for the full transaction and preservation limits.

## rendering and input

model text passes through control-sequence filtering before display. shell output goes through the vt parser instead of being forwarded directly to the host terminal. clipboard escape strings and terminal control payloads from a model cannot become host-terminal commands through the transcript renderer.

ordinary terminal text, including dedicated space events and bracketed paste, is forwarded to the selected pty. the centered pane clears covered cells to the default host background when inheritance is selected. a separate screen and scrollback belong to each pty.

the renderer is intentionally text-only. sixel/kitty graphics, arbitrary terminal extensions, full vim emulation, font selection and independent pane opacity are not implemented. platform support currently targets linux amd64 and arm64; only the native architecture's tests execute locally.
