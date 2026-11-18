local M = {}

function M.setup(opts)
  wraith.commands.create("review.draft", function(ctx)
    local subject = ctx.argument ~= "" and ctx.argument or opts.subject
    wraith.prompt.set("review " .. subject .. " for correctness and missing tests")
  end, "prepare a review prompt")

  wraith.commands.create("review.layout", function()
    local windows = wraith.windows.list()
    wraith.windows.split("vertical")
    wraith.windows.focus(windows[1].id)
  end, "split the conversation and focus its first window")

  wraith.keymap.set("prompt.normal", "<Space>rd", "command.review.draft")
  wraith.keymap.set("chat.normal", "<Space>rl", "command.review.layout")

  local subscription = wraith.events.on("ready", function()
    wraith.notify("review commands loaded")
  end)

  return function()
    wraith.events.off(subscription)
  end
end

return M
