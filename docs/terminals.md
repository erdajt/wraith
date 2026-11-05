# terminal panes

`alt+e` toggles the centered scratch shell. `alt+s` toggles the independent shell on the right. opening a pane focuses it; hiding it preserves its process, current directory, environment and running jobs. `ctrl+\` returns input focus to the prompt while leaving the pane visible. toggle a visible pane off and on to focus it again.

inside a terminal, ordinary keys and `ctrl+c` go to the shell. wraith reserves its configured quit, pane toggle and unfocus bindings. `shift+pageup` and `shift+pagedown` browse the pane's bounded scrollback. editing a suggested block places it in the scratch pane's staging area; it does not write or execute it until you explicitly insert it.

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

shells start in the launch directory unless `terminal.directory` is configured. a `~/` prefix is expanded. changing the shell or directory applies when starting a new pane; `/terminal-restart 1` or `/terminal-restart 2` replaces an existing shell. exiting wraith closes both ptys and terminates their shell process groups.

these are linux ptys with a built-in vt screen renderer. cursor movement, alternate screens, scroll regions, bracketed paste, ansi/256/truecolor text, unicode and device reports are supported. scrollback is bounded; resizing preserves cells without paragraph reflow. image protocols, terminal multiplexing protocols and full emulator equivalence are outside the renderer's scope. use the external editor for your complete neovim setup.

staged commands are inserted at the current shell input position. clear unfinished input first and insert only when the shell is ready; the app cannot infer whether an arbitrary foreground program is accepting a shell command.
