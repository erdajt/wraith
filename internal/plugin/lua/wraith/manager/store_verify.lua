local git = require("wraith.manager.store_git_command")
local operation = require("wraith.manager.store_operation")
local M = {}

function M.check(item, directory, options, completed)
  assert(type(item.commit) == "string", "object verification requires an exact commit")
  local control = operation.new(options, completed)
  control.file("stat", { directory }, function(root)
    assert(root.type == "directory", "managed object must be a directory, not a symlink")
    control.file("stat", { directory .. "/.git" }, function(metadata)
      assert(metadata.type == "directory", "managed object requires its own git directory")
      control.run({ "config", "--local", "--get-all", "remote.origin.url" }, directory, function(source)
        assert(source == item.source.url .. "\n", "managed object source differs; choose a distinct package name")
        control.run({ "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}" }, directory, function(output)
          assert(git.commit(output) == item.commit, "managed object revision differs from its lock")
          control.run({ "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching" }, directory, function(status)
            assert(status == "", "managed object has tracked, untracked or ignored changes; preserve or relocate them before updating")
            control.finish({ state = "completed", commit = item.commit, path = directory })
          end)
        end)
      end)
    end)
  end)
  return control
end

return M
