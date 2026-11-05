# using wraith

wraith runs on linux in a truecolor terminal. install go 1.27 or newer to build, ollama to serve models, and neovim for the default editor. the default shell is zsh; change `shell` if you use something else.

```sh
make install
ollama pull hf.co/filvyb/Qwen3.5-9B-heretic-v2-GGUF:Q5_K_M
wraith doctor
wraith
```

ollama must already be serving requests at the configured address. wraith connects to it; it does not start or reconfigure the daemon. `wraith doctor` checks the editor, shell, endpoint, model and thinking setting. use `ollama ps` during generation to inspect cpu/gpu placement.

## first conversation

write a prompt and press `ctrl+s`. `enter` inserts a newline. `esc` enters normal mode; `i` returns to insert mode. `ctrl+e` opens the draft in your editor. save and exit to return the edited text to wraith. it is still a draft until you send it.

press `tab` to focus the transcript. move with `j/k`, `gg/G`, or `ctrl+u/ctrl+d`. search with `/`, repeat with `n/N`, and select messages with `[/]`. `e` edits the selected assistant response; the original is archived before the edit is saved. `i` returns to the prompt.

enter commands either as `/command` in the prompt followed by `ctrl+s`, or press `:` in normal mode and finish the command with `enter`. `/help` opens the reference.

## commands

| command | behavior |
| --- | --- |
| `/model`, `/models` | choose from installed ollama models with `j/k` and `enter` |
| `/think [auto\|off\|on\|low\|medium\|high]` | select supported native model thinking; no argument opens the picker |
| `/thinking` | show or hide reasoning returned by the model |
| `/settings` | open settings; `e` edits the complete config |
| `/set key value` | validate and save a setting, including nested keys |
| `/theme [name]` | choose phosphor, tokyo, ember, ice or mono |
| `/context` | show estimated usage, measured prompt tokens, facts and pins |
| `/compact` | archive and compact older conversation turns |
| `/cancel` | cancel generation or compaction |
| `/instructions`, `/memory` | edit instructions shared by every chat |
| `/remember text` | append a persistent instruction |
| `/pin text` | retain an exact fact in this chat |
| `/pins`, `/unpin number` | inspect or remove pinned facts |
| `/blocks` | select a fenced code block for external editing |
| `/edit`, `/edit-response` | edit the draft or selected assistant response |
| `/new` | start a saved conversation using the current model |
| `/chats`, `/sessions` | choose a saved conversation |
| `/resume id` | load a specific conversation |
| `/archives` | choose an original transcript to restore as a new chat |
| `/restore filename` | fork a named archive into a new chat |
| `/export [path]` | write the active context, facts and pins as markdown |
| `/retry` | retry the last failed or interrupted request |
| `/terminal-restart 1\|2` | restart the scratch or side shell |
| `/quit` | exit and close child shells; `ctrl+q` also works during generation |

switching models retains the conversation and resets thinking to `auto`. selection is saved in the chat. use `/set model name` to change the configuration default as well. capabilities come from ollama; a model with boolean thinking does not acquire low/medium/high levels through a prompt trick. see [ollama's native thinking protocol](https://docs.ollama.com/capabilities/thinking).

## editing commands

1. open `/blocks` and choose a block.
2. edit it in neovim, or your configured editor.
3. save and exit. wraith saves the snippet and opens the scratch terminal with a staged interpreter command.
4. press `enter` to insert that command into the shell input. review the shell input, then press `enter` again to execute it.

saving alone never executes the block. `e` reopens the staged file; `esc` dismisses the staged command. quitting the editor without saving leaves the block unstaged. a save of unchanged contents still counts as a save. if a saved edit fails validation or persistence, wraith retains its temporary file and shows its path.

shell blocks run through bash, `zsh` blocks through zsh, python through python3, javascript through node, ruby through ruby, and go through `go run`. other languages are saved without an interpreter command. interpreters must be installed separately. snippets remain in the data directory for inspection.

## vim bindings

| area | bindings |
| --- | --- |
| prompt modes | `esc`, `i`, `a`, `I`, `A`, `o`, `O`, `v`, `V` |
| movement | `h j k l`, arrows, `w b e`, `W B E`, `0 ^ $`, `gg G`, `%`, `f F t T` |
| edits | `d c y` with motions, `dd cc yy`, `diw ciw yiw`, `daw caw yaw`, `x X s S D C`, `r`, `J`, `~` |
| history and registers | `u`, `ctrl+r`, `p`, `P` |
| search | `/`, `enter`, `n`, `N` |
| counts | numeric prefixes for motions and common operators |
| menus | `j k`, arrows, `gg/G`, home/end, page up/down, `enter`, `esc` |

this is a modal prompt editor, not an embedded neovim instance. macros, plugins, ex commands, dot-repeat and the complete vim grammar belong in the external editor. undo records buffer changes, including individual inserted characters. `ctrl+e` gives you your full neovim configuration.

## command-line options

`wraith` resumes the most recently saved conversation. use `--new` for a new one, `--session id` for a specific chat, `--model name` to override its model, and `--theme name` for a temporary theme override.

`wraith models`, `wraith sessions`, `wraith config`, and `wraith doctor` inspect the installation. `wraith export id` writes markdown to stdout. flags precede positional subcommands:

```sh
wraith --data-dir /tmp/wraith-review sessions
wraith export 0123456789abcdef01234567 > conversation.md
wraith --demo
wraith --snapshot --width 110 --height 32
```

`--demo` and `--snapshot` use a sample conversation without model requests or saved changes. `--config-dir`, `--data-dir`, and `--cache-dir` isolate a workspace. one interactive writer may own each data directory at a time.
