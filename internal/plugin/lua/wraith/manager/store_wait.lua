local M = {}

function M.finish(id, completed)
  local finished, poll = false, nil
  local function finish(status)
    if finished then return end
    finished = true
    if poll then
      wraith.resource.cancel(poll)
      wraith.resource.release(poll)
    end
    wraith.resource.release(id)
    completed(status)
  end

  local status = wraith.resource.status(id)
  if status.done then finish(status); return end
  poll = wraith.timer.start(20, function()
    local current = wraith.resource.status(id)
    if current.done then finish(current) end
  end, { repeat_ms = 20 })
end

return M
