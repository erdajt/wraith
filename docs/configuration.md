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

`/settings.context` → context opens a number editor immediately. `ctrl+u` clears it; enter saves. escape enters Vim normal mode, where `cc` replaces the number; a second escape cancels. `/set context 1000000` saves an override for the active provider and model. `/set context auto` removes that override. settings show the effective capacity and its model.

context accepts any positive integer representable by the binary. model architecture, provider limits and available memory determine what can actually be used. output reservations and source fragments still have to fit before a request is sent. [default resolution and overrides](context.md#capacity) explain how budgets follow model switches.

| setting | default | effect |
| --- | --- | --- |
| `provider` | `ollama` | active backend profile |
| `providers` | six named profiles | endpoints, credentials, cli commands and model options; [details](providers.md) |
| `host` | `http://127.0.0.1:11434` | fallback Ollama endpoint |
| `auto_start` | `true` | start a refused local Ollama endpoint |
| `model` | qwen3.5 heretic 9b q5 | initial model |
| `context` / `predict` | `8192` / `2048` | automatic local-inference context budget and generated-token budget |
| `providers.<name>.context` | `0` | provider capacity override; zero uses automatic defaults |
| `providers.<name>.model_contexts` | empty | capacity overrides keyed by exact model id |
| `thinking` | `auto` | native model default or supported effort value |
| `threads` / `temperature` | `6` / `0.3` | Ollama inference options; API-specific options belong in provider `extra` |
| `keep_alive` / `timeout` | `2m` / `10m` | Ollama idle retention and request/compaction deadline |
| `environment.scope` | `directory` | default private-variable storage: `conversation`, `directory`, or `global`; existing bindings stay in their scopes |
| `execution.mode` | `agent` | request approval when execution is useful; `blocks` keeps approvals with optional sharing |
| `execution.auto_share` | `false` | share blocks-mode output and scratch snapshots automatically |
| `execution.max_steps` | `8` | approved model commands per user turn, 1–100 |
| `execution.max_jobs` | `4` | concurrent tracked jobs, 1–32 |
| `execution.max_reads` | `16` | output queries per user turn, 1–256 |
| `execution.command_timeout` | `10m` | positive per-job duration, independent of model timeout |
| `execution.capture_bytes` / `execution.tail_bytes` | `8388608` / `65536` | retained prefix and recent tail; [limits](execution.md#limits-and-recovery) |
| `execution.output_context` | `8192` | maximum shared output bytes, 256–65536; further bounded by context |
| `compact.mode` | `ask` | `ask`, `auto` or `manual` |
| `compact.threshold` | `0.9` | compaction threshold, 0.5–0.95 |
| `compact.keep_turns` | `2` | retained user/assistant pairs, 1–20 |
| `compact.budget` | `1536` | extraction/audit output budget, at least 256 |
| `theme` / `themes` | `phosphor` / empty | built-in or custom palette; [details](themes.md) |
| `appearance.background` | `theme` | theme background or host-terminal inheritance |
| `appearance.mouse` / `appearance.thinking` | `true` / `false` | mouse events and initial reasoning visibility |
| `animations` | `true` | activity indicators, welcome eye glow and drifting wisps |
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
| config: `variables/` | directory/global bindings and retained-value redaction journal |
| data: `commands/` | compressed tracked command metadata/output and live recovery journals |
| data: `input/` | bounded compressed prompt/command history per directory |
| data: `snippets/`, `exports/` | edited source and markdown exports |
| cache: `compaction/sessions/` | pre-compaction recovery snapshots |
| cache: `repairs/archives/` | compressed superseded response attempts |
| cache: `edits/` | editor buffers and retained invalid edits |
| cache: `ollama.log` | locally started server logs |

roots default to `~/.config/wraith`, `~/.local/share/wraith` and `~/.cache/wraith`. absolute XDG environment paths and CLI directory overrides are supported. new directories use 0700 and new files use 0600. gzip is compression, not encryption. archives and command records are not automatically deleted.

Wraith does not reserve VRAM or control GPU offload. for Ollama, inspect `ollama ps` during generation and adjust model/context size to leave space for desktop work. cloud requests use the selected account and endpoint; CLI backends use their installed authentication.

[private variables](private-variables.md) documents scoped bindings, comments, editor management and provider redaction. agent mode shares tracked evidence by default unless explicitly hidden; `execution.auto_share` controls blocks and scratch snapshots.
