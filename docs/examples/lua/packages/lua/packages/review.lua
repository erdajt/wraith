return {
  dir = "packages/review",
  name = "review",
  main = "review",
  cmd = "review",
  keys = {
    { "<Space>r", "command.review", mode = "chat.normal", desc = "review changes" },
  },
  opts = {
    prompt = "review the current changes and explain any correctness issues",
  },
}
