# configuration

settings live in `~/.config/wraith/config.yaml`, or `$XDG_CONFIG_HOME/wraith/config.yaml`. the first launch writes defaults. legacy `config.json` is migrated when YAML does not yet exist; the original remains available. omitted values receive defaults, while unknown fields and invalid values are rejected.

`/settings` groups values into model, execution, context, appearance, terminals, and keys/editor categories. `/settings.appearance` opens one directly. press `?` on a setting for its detailed explanation, enter to choose a value, or `e` to edit the complete file. invalid saved edits remain in a recovery file and do not replace the active configuration.

```text
/set editor ["nvim"]
/set shell ["bash", "-i"]
/set context 1000000
/set compact.mode auto
/set execution.auto_share false
/set appearance.background inherit
```

context accepts any positive integer representable by the binary. the custom-capacity field is beside the presets. 8192 is a default, not a maximum. model architecture, provider limits and available memory determine what can actually be used. output reservations and source fragments still have to fit before a request is sent.

| setting | default | effect |
| --- | --- | --- |
| `provider` | `ollama` | active backend profile |
| `providers` | six named profiles | endpoints, credentials, cli commands and model options; [details](providers.md) |
| `host` | `http://127.0.0.1:11434` | fallback Ollama endpoint |
| `auto_start` | `true` | start a refused local Ollama endpoint |
| `model` | qwen3.5 heretic 9b q5 | initial model |
| `context` / `predict` | `8192` / `2048` | positive context and generated-token budgets |
| `thinking` | `auto` | native model default or supported effort value |
| `threads` / `temperature` | `6` / `0.3` | Ollama inference options; API-specific options belong in provider `extra` |
| `keep_alive` / `timeout` | `2m` / `10m` | Ollama idle retention and request/compaction deadline |
| `execution.mode` | `agent` | request approval when execution is useful; `blocks` keeps text suggestions |
| `execution.auto_share` | `false` | share completed tracked output automatically |
| `execution.max_steps` | `8` | approved model commands per user turn, 1–100 |
| `execution.output_context` | `8192` | maximum shared output bytes, 256–65536; further bounded by context |
| `compact.mode` | `ask` | `ask`, `auto` or `manual` |
| `compact.threshold` | `0.9` | compaction threshold, 0.5–0.95 |
| `compact.keep_turns` | `2` | retained user/assistant pairs, 1–20 |
| `compact.budget` | `1536` | extraction/audit output budget, at least 256 |
| `theme` / `themes` | `phosphor` / empty | built-in or custom palette; [details](themes.md) |
| `appearance.background` | `theme` | theme background or host-terminal inheritance |
| `appearance.mouse` / `appearance.thinking` | `true` / `false` | mouse events and initial reasoning visibility |
| `animations` | `true` | activity indicators and rotating welcome art |
| `clipboard` | `[wl-copy]` | clipboard executable and arguments; empty disables external copying |
| `leader` | `space` | prefix for command hints |
| `bindings` / `keys` | see example | leader sequences and direct action bindings |

editor, shell, clipboard and CLI commands are argument arrays. arguments are passed directly, without shell interpolation. the editor receives the temporary filename as its last argument. normal tracked shell runs intentionally use the selected shell executable with `-c`.

an explicit `bindings` map replaces the default leader map. spaces separate sequence keys:

```yaml
leader: space
bindings:
  b: blocks
  s: settings
  c c: compact
  c s: resume
keys:
  output_share: ctrl+g
  terminal_overlay: alt+e
  terminal_split: alt+s
  setting_help: '?'
```

[config.example.yaml](config.example.yaml) is generated from the defaults. [terminal settings](terminals.md) cover pane dimensions, colors, shell and scrollback. fonts, numerical opacity and blur belong to the host terminal; Wraith can inherit its background but cannot assign independent pane alpha.

## storage

| location below the wraith root | contents |
| --- | --- |
| config: `config.yaml`, `instructions.md` | settings and global user preferences |
| data: `sessions/`, `archives/` | compressed active sessions and immutable originals |
| data: `commands/` | compressed tracked command metadata and output |
| data: `input/` | bounded compressed prompt/command history per directory |
| data: `snippets/`, `exports/` | edited source and markdown exports |
| cache: `compaction/sessions/` | pre-compaction recovery snapshots |
| cache: `edits/` | editor buffers and retained invalid edits |
| cache: `ollama.log` | locally started server logs |

roots default to `~/.config/wraith`, `~/.local/share/wraith` and `~/.cache/wraith`. absolute XDG environment paths and CLI directory overrides are supported. new directories use 0700 and new files use 0600. gzip is compression, not encryption. archives and command records are not automatically deleted.

Wraith does not reserve VRAM or control GPU offload. for Ollama, inspect `ollama ps` during generation and adjust model/context size to leave space for desktop work. cloud requests use the selected account and endpoint; CLI backends use their installed authentication.
