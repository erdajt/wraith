# keybindings

normal-mode `:q`, `:qa` and `:quit` close the app. each preserves a partial streamed reply first; if saving fails, the session stays open. `q` in normal mode and the configured quit key use the same behavior.

bindings depend on focus and mode. the header shows both. the footer uses your configured keys. quit and close hints are red, refresh hints blue, and cancel or reject hints yellow. custom palettes can change those colors.

## prompt

| key | action |
| --- | --- |
| `ctrl+s` | send the draft |
| enter | insert a newline; choose a visible slash completion |
| escape | enter normal mode |
| `i`, `a` | insert before or after the cursor |
| `I`, `A` | insert at the line start or line end |
| `o`, `O` | open a line below or above |
| `v`, `V` | character or line visual selection |
| `:w` in normal mode | send the draft |
| `ctrl+e` | edit externally; saving returns the draft without sending |
| up/down in insert mode | recall submitted prompts and slash commands |
| tab | switch to the conversation; complete a slash command when completion is active |
| `q` in normal mode | save and quit |

recall is scoped to the launch directory. moving past the newest recalled entry restores the unsent draft and cursor. private-variable assignments and provider-setting commands are excluded from recall.

## motions and operators

| keys | behavior |
| --- | --- |
| `h j k l`, arrows | move by character or line |
| `w b e`, `W B E` | word and whitespace-delimited word motions |
| `0`, `^`, `$` | line start, first nonblank character, line end |
| `gg`, `G` | buffer start or end |
| `{`, `}` | paragraph motions |
| `%` | matching delimiter |
| `f<char>`, `F<char>` | find a character forward or backward |
| `t<char>`, `T<char>` | move just before or after a found character |
| `/`, enter, `n`, `N` | search, accept, next and previous match |
| `10j`, `100k`, `2ye` | repeat motions or operators with a count |
| `y`, `yy`, `Y` | yank a motion, the current line, or the current line |
| `ye`, `yf,`, `yf(` | yank through a word end, comma or opening parenthesis |
| `d`, `c` | delete or change a motion |
| `dd`, `cc`, `D`, `C` | delete/change a line or through its end |
| `de`, `dfe`, `df(` | delete through a word end, next `e` or opening parenthesis |
| `diw`, `ciw`, `yiw` | operate inside a word |
| `daw`, `caw`, `yaw` | operate around a word |
| `x`, `X`, `s`, `S`, `r` | remove, substitute or replace text |
| `J`, `~` | join lines or change case |
| `u`, `ctrl+r` | undo or redo |
| `p`, `P` | paste the internal register after or before the cursor |

read-only buffers accept navigation and yanks, not destructive edits. this is a documented modal editor, not a complete vim implementation. use the external editor for plugins, macros, dot-repeat and other text objects.

## conversation

press tab to focus the transcript. `/` searches previous prompts and replies; it highlights matches and moves the cursor to them. `n` and `N` repeat the search. prompt search works on the draft instead when that buffer has focus in normal mode.

`[` and `]` select messages. `za` toggles the selected thinking section; clicking its header does the same. thinking is stored separately from the assistant's answer and uses the theme's muted color.

visual selection yanks prose character by character. if a selection touches a command block, the whole command is copied without fences or decoration. enter on a command opens its editor. enter on an assistant message offers its blocks. `/blocks` opens the block list directly.

`ctrl+e` on a user message asks to fork. accepting creates a new conversation with the earlier turns and puts the selected message in the prompt. the original conversation remains saved. assistant edits archive the original before replacing its text.

### conversation windows

with conversation focus, press `ctrl+w`, then a key:

| key | action |
| --- | --- |
| `s` | split above and below |
| `v` | split side by side |
| `h j k l` | focus a window to the left, below, above or right |
| `w` | cycle visible windows |
| `c` | close this window; retain the last window |
| `o` | keep only this window |
| `=` | equalize sibling sizes |
| `+`, `-` | grow or shrink height |
| `>`, `<` | grow or shrink width |

these windows show the same conversation. each keeps its own cursor, scroll, search and selection. the prompt, thinking expansion and messages remain shared. `:q` and `:qa` still quit the app; use `ctrl+w c` to close a window. raw terminals and prompt insert mode keep their own `ctrl+w` behavior.

splits need at least 24 columns and 4 rows per window. resizing can temporarily hide siblings while keeping the focused window visible; expanding restores their layout. there can be at most 32 windows. clicking conversation text focuses that window and positions its cursor.

streaming leaves frozen views in place. wrapping and thinking expansion retain source positions. forks and compaction move removed positions to the nearest retained message and clear selections that include removed content. clearing or resuming a session resets every window's navigation. table values retain their cell positions when a table becomes a narrow field list.

## menus, output and terminals

| area | keys |
| --- | --- |
| pickers | `j/k` or arrows, `/` filter, enter select, escape close |
| settings | `?` explain selected option, `e` edit config |
| pins | `e` edit, `d` remove |
| private variables | `n/e` editor, `g` scope, `d` remove |
| job output | vim navigation, `/` search, visual selection and yanks; `r` refresh |
| selected job | `ctrl+x` cancel |
| private output | `ctrl+g` share with the agent |
| scratch shell | `alt+e` toggle |
| side shell | `alt+s` toggle |
| live terminal | `ctrl+\` return focus to the prompt |
| terminal scrollback | `shift+pageup`, `shift+pagedown` |
| terminal copy mode | three escapes within a second; `i`, `q` or escape returns to live input |

live terminal keys go to the shell except reserved app bindings. in copy mode, text is a stable snapshot; `r` refreshes it. see [terminals](terminals.md) for staging and lifecycle details.

## leader sequences

space opens hints in normal mode. press the sequence shown in the menu, or `/` to filter it.

| sequence | command |
| --- | --- |
| space `b` | blocks |
| space `e` | edit |
| space `h` | help |
| space `i` | instructions |
| space `j` | jobs |
| space `m` | models |
| space `p` | pins |
| space `r` | history |
| space `s` | settings |
| space `t` | think |
| space `v` | thinking |
| space `c a` | archives |
| space `c c` | compact |
| space `c e` | edit-response |
| space `c n` | clear |
| space `c s` | sessions |

## remap

```yaml
leader: ','
bindings:
  b: blocks
  j: jobs
  s a: settings.appearance
  s e: settings.execution
keys:
  send: ctrl+s
  quit: ctrl+q
  editor: ctrl+e
  terminal_overlay: alt+e
  terminal_split: alt+s
  output_share: ctrl+g
  setting_help: '?'
```

an explicit `bindings` map replaces the default leader map. spaces separate sequence keys. conflicting sequence prefixes are rejected. `keys` maps named actions to terminal key strings; see the [complete defaults](config.example.yaml) for every action. fixed vim motions are separate from these action bindings.

yanks fill the internal register even if the external clipboard helper fails. use `clipboard: [wl-copy]` on wayland, `clipboard: [xclip, -selection, clipboard]` on x11, or `clipboard: []` for internal copying only. choose bindings your host terminal actually forwards.

### mode-specific mappings

`keymaps` uses vim notation and named actions. it applies after default bindings. `/keymaps` lists the effective mappings and descriptions. this maps directional focus to your keys while retaining the default split chords:

```yaml
key_timeout: 750
keymaps:
  - {mode: chat.normal, keys: '<C-h>', action: window.left}
  - {mode: chat.normal, keys: '<C-j>', action: window.down}
  - {mode: chat.normal, keys: '<C-k>', action: window.up}
  - {mode: chat.normal, keys: '<C-l>', action: window.right}
  - {mode: chat.normal, keys: '<Space>sa', action: command.settings.appearance}
```

modes are `prompt.insert`, `prompt.normal`, `prompt.visual`, `prompt.operator`, `chat.normal`, `chat.visual`, `chat.operator`, `command`, `search`, `picker`, `output`, `terminal` and `terminal.copy`. `*` explicitly applies everywhere. ordinary mappings do not intercept pending vim counts or operators; use an operator mode only when that is intentional. raw shells and insert mode have no window-chord delay unless you map keys there.

`<C-w>v` means control-w followed by v. `<A-e>`, `<S-Tab>`, `<Space>`, `<CR>`, `<Esc>`, arrows and function keys are supported. literal characters retain case; `<lt>` means a literal less-than sign. a sequence can contain up to 16 keys. terminals may send control-h for Backspace, control-i for Tab and control-m for Enter, so some combinations cannot be distinguished.

`key_timeout` is 50–5000 milliseconds. an exact mapping with longer children waits for another key, then runs at timeout. an unmatched sequence falls back to configured workspace bindings before reaching its original editor. fallback window chords keep their prefix across key presses; expired prefixes replay once. escape cancels a pending sequence. changing focus or installing new mappings discards pending keys. `ctrl+q` and `ctrl+c` remain direct quit/cancel fallbacks; control-c still interrupts a focused shell.

window actions are `window.split_horizontal`, `window.split_vertical`, `window.left`, `window.down`, `window.up`, `window.right`, `window.next`, `window.close`, `window.only`, `window.equalize`, `window.grow_height`, `window.shrink_height`, `window.grow_width` and `window.shrink_width`. other workspace actions are `prompt.send`, `editor.open`, `focus.next`, `terminal.overlay`, `terminal.side`, `app.quit`, `operation.cancel` and `noop`. every listed slash command has a `command.NAME` action, including settings categories. bind `noop` to consume a sequence without running an operation.

Lua can register the same mappings with `wraith.keymap.set(mode, sequence, action_or_function, description)`. custom commands use `command.NAME` actions and appear in completion. see the [plugin API](plugins.md) and [runnable example](examples/lua/init.lua).

conflicting legacy `keys` entries are rejected after normalizing key names. explicit `keymaps` entries override defaults in their mode. mappings cannot replace an active approval dialog or send a draft from another input surface. editor handoffs retain buffered input until the editor returns; focus changes discard stale sequences.
