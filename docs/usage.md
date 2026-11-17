# using wraith

[getting started](getting-started.md) covers installation. build with go 1.27+ on linux. the defaults use ollama, neovim and zsh. change the editor, shell or provider in `~/.config/wraith/config.yaml`.

```sh
make install
ollama pull hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M
wraith
```

wraith starts a local ollama server when its configured loopback connection is refused, then loads the selected model before you send anything. it reuses an existing server and leaves that shared server running on exit. `/model` also loads a newly selected local model immediately. API and CLI backends use their own connection checks; see [providers](providers.md).

## input and navigation

write a prompt and press `ctrl+s`. enter inserts a newline. escape enters normal mode; `i` returns to insert mode. normal-mode `:w` sends the draft. `ctrl+e` opens the prompt in your editor; save and exit to return the edited draft without sending it.

in insert mode, up/down browse submitted prompts and slash commands from the current directory. returning below the newest entry restores your draft and cursor. when typing a new slash command, arrows select completion candidates instead. enter chooses a completion immediately; no send step is required. argument-taking commands leave room for their argument. tab completes a command name.

`/settings.appearance` opens a category directly. subcommands appear in completion only after an exact command name and a dot, such as `/settings.`. settings and model pickers support `/` filtering, arrows or `j/k`, and one enter to select the highlighted result. `?` explains the selected setting.

press tab to focus the conversation. its themed cursor supports motions, visual selection, yanks and search. `/` searches the focused buffer: draft text when the prompt is in normal mode, or saved prompts and replies when the conversation is focused. matches are highlighted, the cursor moves onto the result, and `n`/`N` selects the next/previous match.

`ctrl+e` on a user message asks to fork before editing. accepting retains the earlier turns in a new session and puts that message back in the draft. newer messages disappear from the fork; the original conversation remains saved. editing an assistant response archives its original text first.

## keys

| area | defaults |
| --- | --- |
| prompt modes | `esc`, `i`, `a`, `I`, `A`, `o`, `O`, `v`, `V` |
| movement | `h j k l`, arrows, `w b e`, `W B E`, `0 ^ $`, `gg G`, `{ }`, `%`, `f F t T` |
| editing | `d c y` with motions, `dd cc yy`, `diw ciw yiw`, `daw caw yaw`, `x X s S D C`, `r`, `J`, `~` |
| yanks | `Y` whole line; `ye` to word end; `yf,` through comma; `yf(` through opening parenthesis |
| find operators | `df(`, `dfe`, `de`, and corresponding change/yank operators |
| undo and register | `u`, `ctrl+r`, `p`, `P` |
| search | `/`, enter, `n`, `N` |
| counts | prefixes such as `10j`, `100k`, `2ye` |
| message selection | `[` and `]` |
| thinking | `za` or click its header |
| blocks | `<leader>b`, `/blocks`, or enter on a command under the cursor |
| leader menu | space; configurable nested sequences with hints |
| shell panes | `alt+e` centered scratch shell; `alt+s` right shell; `ctrl+\` returns focus |
| terminal copy mode | triple escape; vim motions, visual selection and search; `i` returns live input |
| output sharing | `ctrl+g` in tracked output or terminal copy mode; hidden when auto sharing is enabled |
| quit | `q` in normal mode, or `ctrl+q` globally |

visual selection in prose copies the selected text. touching a command block selects and copies the entire command, without its frame or markdown fence. yanks populate the prompt register and call the configured clipboard helper. the default is `wl-copy`; use `xclip -selection clipboard` on X11 or an empty array for internal-register-only copies.

this is a modal editor, not a complete embedded neovim instance. macros, plugins, arbitrary text objects and dot-repeat belong in the external editor. `[`, `]` and `za` are conversation actions rather than a claim to implement every vim command.

## commands

| commands | purpose |
| --- | --- |
| `/providers`, `/provider name` | select a backend |
| `/models`, `/model [id]` | choose or directly set a model |
| `/think [value]`, `/thinking` | native effort controls and reasoning visibility |
| `/settings[.category]`, `/set key value`, `/theme [name]` | configure behavior and appearance |
| `/context`, `/compact`, `/cancel` | inspect, compact or cancel model work |
| `/instructions`, `/memory`, `/remember text` | persistent preferences across chats |
| `/pin text`, `/pins`, `/unpin number` | exact per-chat facts; `e` edits and `d` removes in the pins picker |
| `/blocks`, `/history`, `/jobs` | code actions, saved output and current-session jobs |
| `/env`, `/env edit`, `/env conversation\|directory\|global` | scoped private variables and attached descriptions |
| `/edit`, `/edit-response` | editor handoff or user-message fork |
| `/clear`, `/new` | create a clean conversation while keeping the original |
| `/resume [id]`, `/chats`, `/sessions` | resume from the current directory |
| `/archives`, `/restore filename` | recover an archived source into a new chat |
| `/export [path]`, `/retry` | export markdown or retry interrupted generation |
| `/terminal-restart 1\|2`, `/quit` | restart a shell or exit |

## sessions and launch arguments

plain `wraith` opens a new conversation. `wraith a prompt to prepare` places positional text in the input without sending it. `--new` is not supported because new sessions are the default.

```sh
wraith --resume
wraith --resume SESSION_ID
wraith --provider openai --model MODEL_ID
wraith --theme rose-pine
wraith models
wraith sessions
wraith doctor
wraith config
wraith export SESSION_ID > chat.md
```

both `/resume` and `--resume` open a searchable picker when no id is supplied. sessions belong to their original working directory; symlink aliases resolve to the same directory. a different-directory id reports the original path and opens a fresh chat. an invalid id reports `invalid session id` and opens a fresh chat. old sessions without recorded directories remain exportable but cannot be silently assigned to a directory.

flags precede prompt text or subcommands. `--config-dir`, `--data-dir` and `--cache-dir` provide isolated storage. one interactive writer owns each data directory. `--demo` and `--snapshot` use sample content without model calls or saved changes.

older sessions without an original directory can still be exported. export uses the current directory and retained variable journal for redaction without assigning the session a directory or enabling resume.

see [keybindings](keybindings.md) for leader sequences and remapping, [sessions](sessions.md) for forks and recovery, and [troubleshooting](troubleshooting.md) for failure cases.

## launch reference

| option | behavior |
| --- | --- |
| `--resume [id]` | choose a directory-scoped session, or resume its id |
| `--session id` | explicit id form of resume |
| `--provider name` | choose a configured profile |
| `--model id` | choose a model for the launched conversation |
| `--theme name` | choose the launch theme |
| `--config-dir path` | override config and private-variable storage |
| `--data-dir path` | override sessions, jobs and input history |
| `--cache-dir path` | override recovery and temporary editor storage |
| `--demo` | open read-only sample content without model calls |
| `--snapshot` | print one ansi demo frame and exit |
| `--width`, `--height` | snapshot columns and rows; defaults 110 and 34 |
| `--version` | print the build version |
| `--help` | print flag help |

`models`, `sessions`, `doctor`, `config` and `export` are noninteractive subcommands. any other positional text becomes the initial draft. `--snapshot` clamps its dimensions to the app minimum of 50 by 18. development builds identify as `dev`; release packaging embeds the tag version.

## rendered replies

replies render headings, emphasis, inline code, fenced blocks, lists, quotes, links and tables. fenced commands retain their own selection boundaries and editor actions. long content wraps to the available conversation width, including beside the side shell. terminal control sequences from model text are filtered before rendering.

thinking is a separate section, collapsed by default. `/thinking` toggles visibility across the conversation, while `za` or clicking a header toggles one section. `/think` changes generation effort instead. a provider must return reasoning for it to appear.
