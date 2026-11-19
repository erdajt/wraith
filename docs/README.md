# wraith docs

| guide | contents |
| --- | --- |
| [getting started](getting-started.md) | build, install, backend setup, first conversation and diagnostics |
| [usage](usage.md) | prompt editing, conversation navigation, slash commands and launch arguments |
| [keybindings](keybindings.md) | modes, motions, operators, leader sequences and remapping |
| [execution](execution.md) | approval, background jobs, bounded output reads, sharing and recovery |
| [private variables](private-variables.md) | scopes, comments, editor management, substitution and redaction |
| [context](context.md) | capacity, thinking storage, instructions, pins, compaction and archives |
| [sessions](sessions.md) | directory ownership, resume, forks, export and backups |
| [providers](providers.md) | ollama, compatible apis, anthropic, deepseek and cli backends |
| [plugins](plugins.md) | lua setup, API reference, commands, events, reload and runtime limits |
| [lua packages](lua/packages.md) | modular specs, lazy loading, installation, pins, lockfiles and recovery |
| [lua interfaces](lua/surfaces.md) | editable buffers, windows, regions, dialogs and local mappings |
| [lua services](lua/services.md) | managed processes, http, filesystem, timers and json |
| [lua workspace](lua/workspace.md) | bounded session, conversation, history and output queries |
| [lua model extensions](lua/extensions.md) | request policies, custom providers and approved tools |
| [lua examples](lua/examples.md) | executable plugins and configuration patterns |
| [lua editor setup](lua/editor.md) | LuaLS completion, signatures and diagnostics |
| [configuration](configuration.md) | settings categories, validation, defaults and storage |
| [full default config](config.example.yaml) | every serialized setting, provider field and default binding |
| [themes](themes.md) | built-in themes, custom palettes, hint colors and backgrounds |
| [terminals](terminals.md) | pane sizing, shell lifecycle, staging, copy mode and terminal limits |
| [troubleshooting](troubleshooting.md) | connection, model, clipboard, rendering and execution failures |
| [screenshots](screenshots.md) | app captures, fixture provenance and reproduction |
| [architecture](architecture.md) | packages, event loop, persistence, privacy and execution boundaries |
| [development](development.md) | tests, race checks, fuzzing, builds, ci and releases |

start with [getting started](getting-started.md). use `/help` inside wraith for commands and current bindings. `/settings` groups options by purpose; `?` explains the selected setting. `/settings.` completes category names.

## choose the right place

| what you want to keep | where it belongs |
| --- | --- |
| a preference for every conversation | `/instructions` or `/remember` |
| an exact fact for one conversation | `/pin` and `/pins` |
| a value the model should refer to by name | `/env` |
| a model's context capacity | `/set context NUMBER` |
| an app default or binding | `/settings`, `config.yaml` or `init.lua` |
| a custom command or lifecycle callback | a [local lua plugin](plugins.md) |
| installed extensions and their exact revisions | [package specs and lockfile](lua/packages.md) |
| a command's output | `/history`; sharing is a separate choice |
| the complete pre-compaction source | `/archives` |

stored data uses private file permissions. compression is not encryption. model-written summaries are not a substitute for exact archives.
