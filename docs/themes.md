# themes

`/theme` lists built-in and custom palettes. `theme` in config.yaml sets the default. the cursor, selection, search matches, thinking, menus and animated welcome icon use the selected palette.

built-ins: **phosphor**, **tokyo**, **ember**, **ice**, **mono**, **rose-pine**, **dracula**, **everforest**, **nord**, and **catppuccin-mocha**.

custom palettes define seven quoted hex colors:

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
```

`bg` and `fg` set the main surface and text. `muted` colors secondary text and expanded thinking. `accent` drives the cursor and primary actions; `second` colors visual selections. `warn` is used for search matches and warnings. a custom name appears in the picker; reusing a built-in name overrides its palette.

`appearance.background: inherit` uses the host terminal background for the app. `terminal.background: inherit` does the same for shell panes and clears the covered app cells. actual wallpaper visibility depends on the host's transparency. font, alpha percentage and blur must be configured in the terminal or compositor.

upstream palette references: [rose pine](https://rosepinetheme.com/palette/), [dracula](https://github.com/dracula/dracula-theme), [everforest](https://github.com/sainnhe/everforest/blob/master/palette.md), [nord](https://github.com/nordtheme/nord), and [catppuccin](https://github.com/catppuccin/palette).
