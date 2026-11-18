for _, mode in ipairs({ "prompt.normal", "chat.normal", "chat.visual", "chat.operator" }) do
  wraith.keymap.set(mode, "<C-w>s", "window.split_horizontal")
  wraith.keymap.set(mode, "<C-w>v", "window.split_vertical")
  wraith.keymap.set(mode, "<C-w><C-s>", "window.split_horizontal")
  wraith.keymap.set(mode, "<C-w><C-v>", "window.split_vertical")

  for key, action in pairs({
    ["+"] = "grow_height", ["-"] = "shrink_height",
    [">"] = "grow_width", ["<lt>"] = "shrink_width",
    ["="] = "equalize", o = "only", c = "close",
  }) do
    wraith.keymap.set(mode, "<C-w>" .. key, "window." .. action)
  end
end

for _, mode in ipairs({ "prompt.insert", "prompt.normal", "prompt.visual", "prompt.operator", "chat.normal", "chat.visual", "chat.operator" }) do
  for key, direction in pairs({ h = "left", j = "down", k = "up", l = "right" }) do
    wraith.keymap.set(mode, "<C-" .. key .. ">", "focus." .. direction)
  end
end
