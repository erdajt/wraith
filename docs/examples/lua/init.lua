assert(wraith.api_version == 1)

wraith.config.setup({
  theme = "rose-pine",
  key_timeout = 500,
  appearance = { background = "inherit" },
})

for _, mode in ipairs({ "chat.normal", "chat.visual" }) do
  for key, direction in pairs({ h = "left", j = "down", k = "up", l = "right" }) do
    wraith.keymap.set(mode, "<C-" .. key .. ">", "window." .. direction)
  end
end

wraith.keymap.set("chat.normal", "<Space>sv", "window.split_vertical")
wraith.keymap.set("chat.normal", "<Space>ss", "window.split_horizontal")

wraith.theme.register("nightwatch", {
  bg = "#151b22",
  fg = "#d8dee9",
  muted = "#8692a6",
  accent = "#8fbcbb",
  second = "#b48ead",
  border = "#394454",
  warn = "#ebcb8b",
})

wraith.plugins.load("plugins.review", { subject = "the selected changes" })
