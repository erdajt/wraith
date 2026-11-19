local git = require("wraith.manager.store_git_command")
local operation = require("wraith.manager.store_operation")
local M = {}

function M.prepare(item, directory, options, completed)
  local control = operation.new(options, completed)
  local run, finish = control.run, control.finish
  local commit, format, reference
  if item.commit then
    reference, format = item.commit, #item.commit == 64 and "sha256" or "sha1"
  elseif item.selector.kind == "branch" then
    reference = "refs/heads/" .. item.selector.value
  elseif item.selector.kind == "tag" then
    reference = "refs/tags/" .. item.selector.value
  else
    reference = "HEAD"
  end

  local function initialize()
    run({ "--git-dir=" .. directory .. "/.git", "--work-tree=" .. directory,
      "init", "--quiet", "--template=", "--object-format=" .. format, "--", directory }, nil, function()
      run({ "remote", "add", "origin", item.source.url }, directory, function()
        run({ "fetch", "--quiet", "--no-tags", "--depth=1", "--", "origin", reference }, directory, function()
          run({ "rev-parse", "--verify", "--end-of-options", "FETCH_HEAD^{commit}" }, directory, function(output)
            commit = git.commit(output)
            assert(not item.commit or commit == item.commit, "fetched commit differs from the locked revision")
            run({ "checkout", "--quiet", "--detach", commit }, directory, function()
              run({ "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching" }, directory, function(status)
                assert(status == "", "prepared package checkout is not clean")
                finish({ state = "completed", commit = commit, path = directory })
              end)
            end)
          end)
        end)
      end)
    end)
  end

  local function resolve()
    local arguments = { "ls-remote", "--exit-code" }
    if reference ~= "HEAD" then arguments[#arguments + 1] = "--refs" end
    arguments[#arguments + 1], arguments[#arguments + 2], arguments[#arguments + 3] = "--", item.source.url, reference
    run(arguments, nil, function(output)
      local object, ref = output:match("^([a-fA-F0-9]+)\t([^\r\n]+)\n$")
      assert(object and (#object == 40 or #object == 64) and ref == reference, "git did not return the requested ref")
      format = #object == 64 and "sha256" or "sha1"
      initialize()
    end)
  end

  if format then
    initialize()
  elseif reference == "HEAD" then
    resolve()
  else
    run({ "check-ref-format", reference }, nil, resolve)
  end

  return control
end

return M
