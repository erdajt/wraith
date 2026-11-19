local M = {}

function M.new(completed)
  local flow, current, previous, last, finished, cancelled = {}, nil, nil, nil, false, false

  function flow.resource()
    if current then return current.resource and current.resource() or current.id or last end
    return last
  end

  function flow.finish(result)
    if finished then return end
    finished = true
    completed(result)
  end

  function flow.begin(factory, next, settle)
    if cancelled then flow.finish({ state = "cancelled", error = "package operation cancelled" }); return end
    current = factory(function(result)
      last, previous, current = flow.resource(), current, nil
      if not settle and (cancelled or result.state == "cancelled") then
        flow.finish({ state = "cancelled", error = "package operation cancelled" })
        return
      end
      local ok, err = pcall(next, result)
      if not ok then
        if finished then error(err) end
        flow.finish({ state = "failed", error = tostring(err) })
      end
    end)
    last = flow.resource()
  end

  function flow.retained()
    return current and current.retained() or previous and previous.retained() or false
  end

  function flow.abandon()
    cancelled = true
    if current then current.abandon() end
    if previous and previous ~= current then previous.abandon() end
  end

  function flow.cancel()
    cancelled = true
    if current then current.cancel() end
  end

  return flow
end

return M
