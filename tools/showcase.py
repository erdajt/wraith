"""sample content and keyboard walkthrough for real terminal captures."""

import json
import re
import sys
import time

HEADERS = """HTTP/2 200
server: lab-gateway
content-type: text/html; charset=utf-8
strict-transport-security: max-age=31536000
x-content-type-options: nosniff
referrer-policy: strict-origin-when-cross-origin
cache-control: private, no-store
x-frame-options: DENY

fixture notes
  target: app.lab.test
  source: saved local response
  network requests: none
  review: header policy
"""


def provider():
    messages = json.loads(sys.stdin.read().split("Conversation JSON:\n", 1)[1])
    prompt = messages[-1]["content"]
    if "approved background command accepted" in prompt:
        job = re.search(r"id: ([a-f0-9]+)", prompt).group(1)
        answer = '```wraith-tool\n' + json.dumps({"tool": "output", "id": job,
            "pattern": "strict-transport", "ignore_case": True, "limit": 8}) + '\n```'
    elif "command result" in prompt or "output query" in prompt or "tool result" in prompt:
        answer = "## captured evidence\n\nthe local report is available in `/history`. the hsts header sets a one-year max-age. confirm whether the intended policy also requires subdomains.\n\n**next:** compare the saved response with the deployment policy. keep any network verification as a separate approval."
    elif "run the saved report" in prompt:
        answer = "read the saved response and keep this chat available while the job finishes.\n\n```wraith-run\n" + json.dumps({"command": "cat headers.txt; sleep 15; printf '\\nreview complete\\n'",
            "reason": "inspect the local header fixture; no request is sent to the target"}) + "\n```"
    else:
        answer = "## header review\n\n**wraith::TARGET_DOMAIN** · saved response · read-only scope\n\n| header | observed | review |\n| --- | --- | --- |\n| hsts | 1 year | check subdomain policy |\n| content type | nosniff | present |\n| framing | DENY | present |\n| csp | absent | compare intended policy |\n\n### verify separately\n\n```sh\ncurl -sSI wraith::TARGET_URL \\\n  | grep -Ei 'strict-transport|content-security|x-frame'\n```\n\nthis is a **suggested command**. open the block to edit it before running.\n\n- preserve the original headers as evidence\n- inspect deployment config before changing policy\n- keep network checks separate from this local review"
    thinking = "compare the saved headers with the stated policy. distinguish an absent header from a confirmed finding. preserve the source and require separate approval for a network check."
    print(json.dumps({"type": "stream_event", "event": {"delta": {"thinking": thinking}}}), flush=True)
    for start in range(0, len(answer), 36):
        print(json.dumps({"type": "stream_event", "event": {"delta": {"text": answer[start:start + 36]}}}), flush=True)
        time.sleep(0.015)
    print(json.dumps({"type": "result"}), flush=True)


def walkthrough(send, command, capture, pump):
    def press(keys):
        for key in keys:
            send(key, 0.12)

    pump(2)
    capture("welcome", "responsive hooded mark in rose-pine")
    for entry in ("TARGET_DOMAIN=app.lab.test # isolated application under review",
                  "TARGET_URL=https://app.lab.test # separate approval for network checks"):
        command("/env set " + entry)
        send("\x1b")
    command("/pin scope: saved local headers; network checks require separate approval")
    command("/remember distinguish observations from confirmed findings")
    command("review the saved headers; show the evidence and a separate verification command")
    pump(1)
    capture("conversation", "rendered header review with private substitution and a suggested command")
    press("\t]za")
    capture("thinking", "expanded reasoning and the conversation cursor")
    press("za/header\r")
    capture("search", "highlighted conversation matches with the cursor at a result")
    press("v2e")
    capture("selection", "character selection in the conversation")
    press("y ")
    capture("leader", "leader sequences for workspace actions")
    press("\x1b\t")
    command("/blocks")
    capture("blocks", "suggested commands remain available for editing or tracked runs")
    send("\x1b")
    send("compare this report with the deployment policy")
    send("\x05", 0.8)
    capture("editor", "actual neovim editing the prompt")
    send("\x1b:wq\r", 0.7)
    send("\x1b")
    press("ddi")
    command("run the saved report")
    pump(1)
    capture("approval", "command, reason and six explicit approval choices")
    press("jjjj\r")
    send("show only the security headers and keep the original file unchanged")
    capture("modify", "inline command revision with a modal editor")
    send("\x1b")
    send("\x1b")
    send("\r", 1)
    command("/jobs")
    capture("jobs", "background status and stable job id")
    send("\r")
    send("/strict-transport\r")
    capture("output", "live job output with a highlighted search result")
    send("\x1b")
    for name, cmd, caption in [
        ("variables", "/env", "private names, local values and shared comments"),
        ("pins", "/pins", "exact scope fact with edit and remove controls"),
        ("models", "/models", "configured sample model catalog"),
        ("providers", "/providers", "local, api and cli profiles"),
        ("settings", "/settings", "settings categories"),
        ("appearance", "/settings.appearance", "appearance controls"),
        ("context-settings", "/settings.context", "model capacity and compaction policy"),
    ]:
        command(cmd)
        capture(name, caption)
        send("\x1b")
    command("/settings.execution")
    send("?")
    capture("setting-help", "searchable per-setting explanation")
    send("\x1b")
    send("\x1b")
    send("/settings.")
    capture("completion", "dotted category completion")
    send("\x1b")
    press("ddi")
    command("/resume")
    capture("sessions", "directory-scoped session picker")
    send("\x1b")
    command("/clear")
    command("review the saved headers; show the evidence and a separate verification command")
    pump(1)
    send("\x1bs", 0.6)
    send("printf '\\033[35mSAVED RESPONSE\\033[0m\\n'; cat headers.txt\r")
    send("\x1c")
    press("\tgg/header\r")
    capture("workspace", "populated workspace with search, private values, markdown and an independent shell")
    send("\t")
    for theme in ("phosphor", "rose-pine", "dracula", "everforest", "nord", "catppuccin-mocha", "tokyo", "ember", "ice", "mono"):
        command("/theme " + theme)
        press("\tgg/header\r")
        capture("theme-" + theme, theme + " with the same populated workspace and search")
        send("\t")
    command("/theme rose-pine")
    send("\x1bs")
    send("\x1be", 0.6)
    send("cat headers.txt\r")
    capture("scratch-shell", "centered shell with saved local evidence")
    for _ in range(3):
        send("\x1b", 0.15)
    press("gg/strict-transport\rv2e")
    capture("terminal-copy", "search and visual selection in terminal scrollback")
    send("y")
    send("i")
    send("\x1be")
    press("\tgg\x05")
    capture("fork", "user-message editing asks to fork while preserving the original")
    press("\x1b\t")
    command("/context")
    capture("context", "active context estimate and output reservation")
