# screenshots

these are captures of the running app in ghostty on hyprland. each image is cropped to the complete terminal grid: 140 columns by 48 rows. host padding is removed. the app content is not redrawn, recolored or rearranged.

the `sample` provider returns fixed documentation content. the report command really runs local `printf` and `sleep`; it does not contact a target. model names, reasoning, tables and report text are fixtures. displayed token rates are fixture timings, not model benchmarks. no api credentials or existing conversations are used.

## conversation and thinking

![conversation with markdown, a command block, a table and a resolved private variable](assets/screenshots/conversation.png)

markdown headings, emphasis, inline code, fenced commands and tables render inside the conversation. `lab.example` is resolved locally from a private variable.

<details>
<summary>expanded thinking</summary>

![expanded reasoning in muted text with the conversation cursor](assets/screenshots/thinking.png)

`za` or the thinking header toggles reasoning returned by the provider.

</details>

## command approval and jobs

![approval panel with command, reason and run, hide, edit, modify and reject choices](assets/screenshots/approval.png)

approval identifies the working directory and background mode before execution.

| current jobs | live output |
| --- | --- |
| [![job list with status and id](assets/screenshots/jobs.png)](assets/screenshots/jobs.png) | [![live output with vim reader controls](assets/screenshots/output.png)](assets/screenshots/output.png) |

these views come from the local sample job. `/jobs` opens status; enter opens output. the reader supports navigation, search, selection, copying, refresh and cancellation.

## terminal panes

![side shell with a separate prompt and conversation](assets/screenshots/side-shell.png)

`alt+s` opens the side shell. it stops above the app footer.

| scratch shell | terminal copy mode |
| --- | --- |
| [![centered scratch shell](assets/screenshots/scratch-shell.png)](assets/screenshots/scratch-shell.png) | [![terminal snapshot with copy and search controls](assets/screenshots/terminal-copy.png)](assets/screenshots/terminal-copy.png) |

`alt+e` opens the centered shell. three escapes switch its scrollback to a stable vim reader.

## variables, pins and context

| private variables | exact pins |
| --- | --- |
| [![directory variables with comments and scope controls](assets/screenshots/variables.png)](assets/screenshots/variables.png) | [![pins with edit and remove bindings](assets/screenshots/pins.png)](assets/screenshots/pins.png) |

[context usage](assets/screenshots/context.png) appears in the status line after `/context`; the header keeps an estimated usage meter visible throughout the session.

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

click any image for its full-size capture. the welcome mark and wordmark are rendered by the app. animations are disabled for these stills; enabling them moves light across the mark and changes the eye glow without shifting its shape.

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

from a hyprland session with ghostty, grim, python 3, bash and go installed:

```sh
python3 tools/screenshots.py
```

the script builds wraith, opens one floating ghostty window, drives the app through these workflows and closes it. keep the window visible and avoid typing into it during capture. output goes to `/tmp/wraith-screenshots`; use `--output PATH` to change that. temporary config, sessions and shell working directories are isolated and removed afterward.

`--record` records pty output without accessing the desktop. it validates the interaction sequence, but does not create screenshots. `.ansi` files contain the accumulated terminal stream at each checkpoint.

[manifest.json](assets/screenshots/manifest.json) records the binary hash, source commit, fixture description, grid dimensions, raw capture hashes, cropped hashes and crop geometry. the committed pngs preserve the original pixels within that rectangle. the first welcome capture was excluded because keyboard echo appeared in its footer; the theme captures show the same empty-conversation view.

for this capture's 1440 by 960 host window, the app grid starts at `(16, 16)` and measures 1400 by 912 pixels. reproduce its padding crop with:

```sh
magick /tmp/wraith-screenshots/conversation.png \
  -crop 1400x912+16+16 +repage conversation.png
```

measure again if font, scale, window dimensions or padding changes. do not apply a guessed crop that removes app rows. terminal fonts, palette inheritance and compositor transparency can change appearance on another setup.
