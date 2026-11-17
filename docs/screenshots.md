# screenshots

these are captures of the running app in ghostty on hyprland. full captures are cropped to the complete terminal grid: 130 columns by 36 rows. host padding is removed. panel details are tighter crops of those same captures. click a detail to open its full frame. app content is not redrawn, recolored or rearranged.

the `sample` provider returns fixed documentation content. the report command really runs local `cat`, `sleep` and `printf`; it does not contact a target. model names, reasoning, tables and report text are fixtures. the fixture reports no token usage; zero counters are not model benchmarks. no api credentials or existing conversations are used.

## workspace

![conversation search beside a shell showing the saved response](assets/screenshots/workspace.png)

the side shell stays above the footer. the conversation shows markdown, a suggested command, locally resolved values and matching search terms.

## conversation and thinking

![conversation with markdown, a command block, a table and a resolved private variable](assets/screenshots/conversation.png)

markdown headings, emphasis, inline code, fenced commands and tables render inside the conversation. `app.lab.test` is resolved locally from a private variable.

<details>
<summary>expanded thinking</summary>

![expanded reasoning in muted text with the conversation cursor](assets/screenshots/thinking.png)

`za` or the thinking header toggles reasoning returned by the provider.

</details>

## search, selection and editing

| conversation search | visual selection |
| --- | --- |
| [![highlighted matches](assets/screenshots/search.png)](assets/screenshots/search.png) | [![vim cursor and selection](assets/screenshots/selection.png)](assets/screenshots/selection.png) |

normal-mode `/` searches the focused buffer. `n` and `N` move between results. `v` selects characters and `y` copies them; selecting part of a command yanks the whole command.

[leader hints](assets/screenshots/leader.png) show configured sequences. [neovim](assets/screenshots/editor.png) edits the draft through the configured editor command. [fork confirmation](assets/screenshots/fork.png) protects the original chat before editing a previous user message.

## command approval and jobs

[![approval panel with command, reason and run, hide, edit, modify and reject choices](assets/screenshots/approval-detail.png)](assets/screenshots/approval.png)

approval identifies the working directory and background mode before execution. [suggested blocks](assets/screenshots/blocks.png) remain separate from explicit requests.

<details>
<summary>ask the assistant to revise a command</summary>

[![inline command revision](assets/screenshots/modify-detail.png)](assets/screenshots/modify.png)

type a revision here or leave it empty to prepare the next prompt.

</details>

| current jobs | live output |
| --- | --- |
| [![job list with status and id](assets/screenshots/jobs.png)](assets/screenshots/jobs.png) | [![live output with vim reader controls](assets/screenshots/output.png)](assets/screenshots/output.png) |

these views come from the local sample job. `/jobs` opens status; enter opens output. the reader supports navigation, search, selection, copying, refresh and cancellation.

## terminal panes

![side shell with a separate prompt and conversation](assets/screenshots/workspace.png)

`alt+s` opens the side shell. it stops above the app footer.

| scratch shell | terminal copy mode |
| --- | --- |
| [![centered scratch shell](assets/screenshots/scratch-shell.png)](assets/screenshots/scratch-shell.png) | [![terminal snapshot with copy and search controls](assets/screenshots/terminal-copy.png)](assets/screenshots/terminal-copy.png) |

`alt+e` opens the centered shell. three escapes switch its scrollback to a stable vim reader.

## variables, pins and context

| private variables | exact pins |
| --- | --- |
| [![directory variables with comments and scope controls](assets/screenshots/variables.png)](assets/screenshots/variables.png) | [![pins with edit and remove bindings](assets/screenshots/pins.png)](assets/screenshots/pins.png) |

[context settings](assets/screenshots/context-settings.png) cover capacity, reserves and compaction policy. [context usage](assets/screenshots/context.png) appears in the status line after `/context`; the header keeps an estimated usage meter visible throughout the session.

## settings and completion

![categorized settings with help and editor bindings](assets/screenshots/settings.png)

| category completion | appearance settings |
| --- | --- |
| [![slash completion for settings categories](assets/screenshots/completion.png)](assets/screenshots/completion.png) | [![appearance options](assets/screenshots/appearance.png)](assets/screenshots/appearance.png) |

[setting help](assets/screenshots/setting-help.png) explains the selected option when you press `?`.

## providers, models and sessions

| provider profiles | model catalog |
| --- | --- |
| [![provider picker](assets/screenshots/providers.png)](assets/screenshots/providers.png) | [![sample model picker](assets/screenshots/models.png)](assets/screenshots/models.png) |

[the session picker](assets/screenshots/sessions.png) lists saved chats from the current directory. all three pickers support arrows, `j/k`, search and enter.

## themes

every palette shows the same populated workspace. compare the background, borders, command syntax, search highlights, cursor and key hints. click any image for its full-size capture.

the [welcome mark](assets/screenshots/welcome.png) is also rendered by the app. animations are disabled for these stills; enabling them moves light across the mark and changes the eye glow without shifting its shape. [custom palettes](themes.md) use the same rendering paths.

| phosphor | rose-pine |
| --- | --- |
| [![phosphor](assets/screenshots/theme-phosphor.png)](assets/screenshots/theme-phosphor.png) | [![rose-pine](assets/screenshots/theme-rose-pine.png)](assets/screenshots/theme-rose-pine.png) |

| dracula | everforest |
| --- | --- |
| [![dracula](assets/screenshots/theme-dracula.png)](assets/screenshots/theme-dracula.png) | [![everforest](assets/screenshots/theme-everforest.png)](assets/screenshots/theme-everforest.png) |

| nord | catppuccin-mocha |
| --- | --- |
| [![nord](assets/screenshots/theme-nord.png)](assets/screenshots/theme-nord.png) | [![catppuccin-mocha](assets/screenshots/theme-catppuccin-mocha.png)](assets/screenshots/theme-catppuccin-mocha.png) |

| tokyo | ember |
| --- | --- |
| [![tokyo](assets/screenshots/theme-tokyo.png)](assets/screenshots/theme-tokyo.png) | [![ember](assets/screenshots/theme-ember.png)](assets/screenshots/theme-ember.png) |

| ice | mono |
| --- | --- |
| [![ice](assets/screenshots/theme-ice.png)](assets/screenshots/theme-ice.png) | [![mono](assets/screenshots/theme-mono.png)](assets/screenshots/theme-mono.png) |

## reproduce

from a hyprland session with ghostty, grim, python 3, bash, neovim, go and the JetBrainsMono Nerd Font Mono font installed:

```sh
python3 tools/screenshots.py --output /tmp/wraith-showcase
```

the script builds wraith, opens one floating ghostty window, drives the app through these workflows and closes it. keep the window visible and avoid typing into it during capture. the example writes to `/tmp/wraith-showcase`; without `--output`, the default is `/tmp/wraith-screenshots`. temporary config, sessions and shell working directories are isolated and removed afterward.

`--record` records pty output without accessing the desktop. it records the interaction sequence for inspection, but does not assert checkpoint contents or create screenshots. `.ansi` files contain the accumulated terminal stream at each checkpoint.

[manifest.json](assets/screenshots/manifest.json) records the binary hash, source commit, fixture description, grid dimensions, dirty working-tree state, raw capture hashes, cropped hashes and crop geometry. source identity is recorded when the app starts, so later commits cannot relabel the capture. the commit identifies head; `source_dirty` records local edits at capture time. the binary hash identifies the build used. the committed pngs preserve the original pixels within their crop rectangles.

for this capture's 1440 by 900 host window with a 14 pt font, the app grid starts at `(0, 0)` and measures 1430 by 900 pixels. reproduce its padding crop with:

```sh
magick /tmp/wraith-showcase/conversation.png \
  -crop 1430x900+0+0 +repage conversation.png
```

`grid_crop` applies to full frames. detail entries override it with `[x, y, width, height]` and name their source capture.

measure again if font, scale, window dimensions or padding changes. do not apply a guessed crop that removes app rows. terminal fonts, palette inheritance and compositor transparency can change appearance on another setup.
