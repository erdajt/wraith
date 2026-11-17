# themes

`/theme` lists built-in and custom palettes. `theme` in config.yaml sets the default. the cursor, selection, search matches, thinking, menus and animated welcome icon use the selected palette.

built-ins: **phosphor**, **tokyo**, **ember**, **ice**, **mono**, **rose-pine**, **dracula**, **everforest**, **nord**, and **catppuccin-mocha**.

custom palettes require seven quoted hex colors. three optional hint colors are shown below:

```yaml
theme: dusk
themes:
  dusk:
    bg: '#15131d'
    fg: '#eee8f5'
    muted: '#9a8fa8'
    accent: '#c5a5ff'
    second: '#80dfce'
    border: '#42384e'
    warn: '#ffc28b'
    hint_quit: '#f38ba8'
    hint_refresh: '#89b4fa'
    hint_cancel: '#f9e2af'
```

`bg` and `fg` set the main surface and text. `muted` colors secondary text and expanded thinking. `accent` drives the cursor and primary actions; `second` colors visual selections. `warn` is used for search matches and warnings. a custom name appears in the picker; reusing a built-in name overrides its palette.

keybinding hints use red for quit and close, blue for refresh, and yellow for cancel, reject and reset. other bindings use `accent`. optional `hint_quit`, `hint_refresh` and `hint_cancel` colors override these defaults in a custom palette. existing palettes without these fields keep the default hint colors. the action determines its color, including when its key is rebound.

`appearance.background: inherit` uses the host terminal background for the app. `terminal.background: inherit` does the same for shell panes and clears the covered app cells. actual wallpaper visibility depends on the host's transparency. font, alpha percentage and blur must be configured in the terminal or compositor.

upstream palette references: [rose pine](https://rosepinetheme.com/palette/), [dracula](https://github.com/dracula/dracula-theme), [everforest](https://github.com/sainnhe/everforest/blob/master/palette.md), [nord](https://github.com/nordtheme/nord), and [catppuccin](https://github.com/catppuccin/palette).

## welcome and motion

the empty conversation shows a hooded mark and a block wordmark. it uses `second` for the hood, `accent` for the eyes and lettering, and `fg` for the brighter eye phase. animation moves a light band over the mark without changing its silhouette. shorter terminals use a compact mark and wordmark.

```text
/set animations false
/set appearance.background inherit
```

disabling animation freezes the welcome frame. background inheritance applies to the terminal surface, not an independent alpha layer. [the gallery](screenshots.md#themes) shows every built-in palette in the actual app; [configuration](configuration.md) covers persistence and validation.
