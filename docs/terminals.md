# terminal panes

`alt+e` toggles the centered scratch shell. `alt+s` toggles the independent shell on the right. opening a pane focuses it; hiding it preserves its process, current directory, environment and running jobs. `ctrl+\` returns input focus to the prompt while leaving the pane visible. toggle a visible pane off and on to focus it again.

inside a terminal, ordinary keys and `ctrl+c` go to the shell. wraith reserves its configured quit, pane toggle and unfocus bindings. `shift+pageup` and `shift+pagedown` browse the pane's bounded scrollback. saving an edited suggestion inserts it for review when shell integration is available; it never executes on save.

## copy mode

press escape three times within a second to open a stable scrollback snapshot. use vim motions, counts, `/`, `n`/`N`, `v`/`V`, `y`, `Y`, `ye` and `yf<char>` to browse and copy. search highlights matches and moves the cursor to them. `r` refreshes the snapshot; `i`, `q` or escape returns to live input. the first two escape keys still reach the foreground program.

## appearance

```text
/set terminal.background inherit
/set terminal.foreground inherit
/set terminal.overlay_width 0.7
/set terminal.overlay_height 0.6
/set terminal.split_width 0.4
/set terminal.scrollback 5000
/set shell ["zsh", "-i"]
```

`inherit` emits the terminal's default background or foreground. wraith clears the app content underneath the pane before drawing it. if your host terminal makes its default background transparent, the wallpaper can show through. an opaque host terminal remains opaque.

`theme` uses the active wraith palette. an explicit `#rrggbb` color is also accepted. ansi palette colors from shell applications use the host terminal's palette; explicit truecolor output retains the application's requested colors.

fonts, font size, opacity, blur and wallpaper compositing belong to the host terminal and compositor. a text-mode application cannot select a different font or force real per-pane opacity. configure those in ghostty, kitty, wezterm or your terminal's settings. wraith does not pretend that a black background is transparency.

## shell lifecycle

shells start in the launch directory unless `terminal.directory` is configured. a `~/` prefix is expanded. changing the shell or directory applies when starting a new pane; `/terminal-restart 1` or `/terminal-restart 2` replaces an existing shell. typing `exit` automatically starts a replacement shell in the same pane. three rapid restarts stop the loop if shell startup keeps failing; use `/terminal-restart` after correcting the shell configuration. hidden panes remain hidden during replacement.

exiting wraith closes both ptys and terminates their shell process groups.

these are linux ptys with a built-in vt screen renderer. cursor movement, alternate screens, scroll regions, bracketed paste, ansi/256/truecolor text, unicode and device reports are supported. scrollback is bounded; resizing preserves cells without paragraph reflow. image protocols, terminal multiplexing protocols and full emulator equivalence are outside the renderer's scope. use the external editor for your complete neovim setup.

## saved-command insertion

`shell: ["zsh"]` and `shell: ["zsh", "-i"]` support automatic insertion. a temporary startup proxy loads your existing `.zshenv`, honors its `ZDOTDIR` changes and leaves subsequent startup files in their normal order. it registers a zsh line-editor widget through `precmd_functions`; a startup configuration that replaces that hook can disable integration. no persistent shell files are edited.

the widget receives a saved command through dedicated pipes and a private temporary file. it inserts only when the line editor is active and both the current buffer and continuation buffer are empty. it assigns text without evaluating it or accepting the line. multiline text is preserved. foreground programs never receive the command through terminal input. the snippet remains on disk under the app's snippets directory.

if input is occupied, the snippet stays staged. `ctrl+c` interrupts shell input; enter retries insertion through the same empty-buffer check. a second save retains its new snippet separately and reports its path instead of overwriting the pending command.

escape requests cancellation while waiting for acknowledgement. after three seconds without acknowledgement, wraith requests cancellation and retains the snippet. cancellation can race with insertion, so a late acknowledgement may report that the text was already inserted. wraith blocks reinsertion while the outcome is unknown. `ctrl+c` can interrupt a foreground program. `/terminal-restart 1` or `2` resets the shell and preserves the snippet for explicit insertion; failed restarts retain its path too.

bash, other shells, wrappers and custom zsh arguments currently use manual staging. enter inserts at the current input position without executing; the next enter executes. clear unfinished input first and insert only at a shell prompt. manual multiline insertion requires bracketed paste. terminal output alone cannot prove that an arbitrary foreground program is ready for a shell command.
