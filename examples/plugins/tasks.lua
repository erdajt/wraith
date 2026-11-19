return {
  setup = function(options)
    local tasks = options.tasks or {
      { id = "review", label = "review changes", prompt = "review the changes, identify defects and cite the affected files" },
      { id = "investigate", label = "investigate a failure", prompt = "inspect the evidence, isolate the cause and verify the fix" },
    }
    local selected, header

    wraith.commands.create("task", function()
      local items = {}
      for _, task in ipairs(tasks) do
        items[#items + 1] = { id = task.id, label = task.label }
      end
      wraith.ui.select(items, { title = "workspace task" }, function(ctx)
        if ctx.data.cancelled then return end
        selected = tasks[ctx.data.index]
        if header then
          wraith.regions.set(header, { lines = { "task: " .. selected.label } })
        else
          header = wraith.regions.create({ slot = "header", lines = { "task: " .. selected.label } })
        end
        wraith.events.emit("tasks.selected", { id = selected.id })
      end)
    end, "choose the request context")

    wraith.hooks.add("before_request", function(ctx)
      if not selected then return end
      table.insert(ctx.request.messages, 1, { role = "user", content = "workspace task: " .. selected.prompt })
      return { request = ctx.request }
    end)
  end,
}
