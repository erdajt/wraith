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
  if status.done then finish(status); return function() return id end end
  poll = wraith.timer.start(20, function()
    local current = wraith.resource.status(id)
    if current.done then finish(current) end
  end, { repeat_ms = 20 })
  return function() return poll or id end
end

function M.retained(id, waiting)
  if pcall(wraith.resource.status, id) then return true end
  local receipt = waiting and waiting()
  return receipt ~= nil and receipt ~= id and pcall(wraith.resource.status, receipt) or false
end

function M.abandon(id, waiting)
  pcall(wraith.resource.cancel, id)
  pcall(wraith.resource.release, id)
  local receipt = waiting and waiting()
  if receipt and receipt ~= id then
    pcall(wraith.resource.cancel, receipt)
    pcall(wraith.resource.release, receipt)
  end
end

return M
