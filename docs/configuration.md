# configuration

settings live in `~/.config/wraith/config.json`, or `$XDG_CONFIG_HOME/wraith/config.json`. the first launch writes defaults. [config.example.json](config.example.json) contains the complete schema. omitted fields receive defaults; unknown fields, malformed trailing data and invalid values are rejected.

use `/settings` for common choices, press `e` there to edit the entire file, or set a value directly:

```text
/set editor ["nvim"]
/set shell ["bash", "-i"]
/set theme tokyo
/set context 16384
/set predict 2048
/set threads 6
/set compact.mode auto
/set compact.threshold 0.9
/set keys.terminal_overlay alt+x
/set animations false
```

editor and shell values are argument arrays. arguments are passed directly to the executable, without shell expansion or `eval`. use an absolute executable path if it is outside `PATH`. editor arguments precede the temporary file path. neovim is the default regardless of `$EDITOR`; set the array explicitly to choose another editor.

| setting | default | constraint or effect |
| --- | --- | --- |
| `host` | `http://127.0.0.1:11434` | native ollama endpoint; http(s), no embedded credentials |
| `model` | qwen3.5 heretic q5 model in the example | new-chat default; `/models` selects from installed models |
| `context` | `8192` | 2048–262144; requested context, not a guarantee of model capacity |
| `predict` | `2048` | at least 128, at most half the context; includes native thinking output |
| `threads` | `6` | 0 uses ollama's default; otherwise 1–512 cpu inference threads |
| `temperature` | `0.3` | 0–2 |
| `thinking` | `auto` | native supported value; validated when sent |
| `theme` | `phosphor` | phosphor, tokyo, ember, ice, mono |
| `keep_alive` | `2m` | positive duration; request that ollama unload an idle model after this interval |
| `timeout` | `10m` | positive duration for each generation or whole compaction operation |
| `compact.mode` | `ask` | ask, auto or manual |
| `compact.threshold` | `0.9` | 0.5–0.95 of the configured context |
| `compact.keep_turns` | `2` | retain 1–20 recent user/assistant pairs |
| `compact.budget` | `1536` | per extraction/audit output budget; 256 to half the context |
| `animations` | `true` | animate the activity indicator; false reduces redraw cadence |

terminal appearance and dimensions are documented in [terminals.md](terminals.md). bindings use terminal key names such as `ctrl+s`, `alt+e`, `tab`, and `ctrl+\`. all global action bindings must be nonempty and distinct. choose keys your host terminal actually forwards. the prompt's modal grammar remains fixed; your external editor can provide arbitrary remaps.

## resource use

8192 context and 2048 output are starting defaults for the selected 9b q5 model and 12 gb gpu. they are not a benchmark or vram reservation. model weights, kv cache, context length, concurrent requests and the desktop share gpu memory. wraith makes one inference request at a time; compaction extraction and audit requests are sequential.

increase context in steps and check `ollama ps` while generating. if it reports cpu execution or mixed offload, fix ollama's gpu backend or reduce model/context size. wraith cannot turn a cpu-only ollama installation into a cuda installation. its thread setting also cannot reserve gpu compute for games. a short keep-alive releases an idle model sooner; `ollama stop MODEL` unloads it explicitly.

## files and overrides

| location | contents |
| --- | --- |
| `~/.config/wraith/config.json` | validated settings |
| `~/.config/wraith/instructions.md` | global user instructions, up to 64 kib |
| `~/.local/share/wraith/sessions/` | active conversations as gzip json |
| `~/.local/share/wraith/archives/` | immutable original transcripts as gzip json |
| `~/.local/share/wraith/snippets/` | edited source files staged for execution |
| `~/.local/share/wraith/exports/` | default markdown export destination |
| `~/.cache/wraith/compaction/sessions/` | latest pre-compaction recovery snapshot per chat |
| `~/.cache/wraith/edits/` | editor buffers; failed saved edits are retained |

`XDG_DATA_HOME` and `XDG_CACHE_HOME` override their corresponding roots. only absolute xdg paths are used. command-line directory overrides take precedence. newly created directories use mode 0700; files use 0600. gzip provides compression, not encryption. archives have no automatic deletion policy.
