local M = {}

function M.setup(opts)
  assert(type(opts.prompt) == "string", "review requires a prompt")

  wraith.commands.create("review", function(ctx)
    local prompt = opts.prompt
    if ctx.argument ~= "" then
      prompt = prompt .. "\n\nfocus: " .. ctx.argument
    end
    wraith.prompt.set(prompt)
  end, "prepare a review prompt", { arguments = true })
end

return M
