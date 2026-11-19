local process = require("wraith.manager.store_process")
local M = {}
local policy = {
  "core.hooksPath=/dev/null", "core.fsmonitor=false", "submodule.recurse=false",
  "protocol.allow=never", "protocol.https.allow=always", "protocol.ssh.allow=always", "protocol.file.allow=always",
}

function M.run(arguments, directory, options, completed)
  local argv = { "git" }
  for _, setting in ipairs(policy) do
    argv[#argv + 1], argv[#argv + 2] = "-c", setting
  end
  if directory then argv[#argv + 1], argv[#argv + 2] = "-C", directory end
  for _, value in ipairs(arguments) do argv[#argv + 1] = value end
  return process.start(argv, options, completed)
end

function M.commit(output)
  local value = output:match("^([a-fA-F0-9]+)\n$")
  assert(value and (#value == 40 or #value == 64), "git did not return one complete commit id")
  return value:lower()
end

return M
