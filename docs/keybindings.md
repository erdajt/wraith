# keybindings

bindings depend on focus and mode. the header shows both. the footer uses your configured keys. quit and close hints are red, refresh hints blue, and cancel or reject hints yellow. custom palettes can change those colors.

## prompt

| key | action |
| --- | --- |
| `ctrl+s` | send the draft |
| enter | insert a newline; choose a visible slash completion |
| escape | enter normal mode |
| `i`, `a` | insert before or after the cursor |
| `I`, `A` | insert at the first nonblank character or line end |
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
