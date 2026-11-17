#!/usr/bin/env python3
"""capture wraith in ghostty on hyprland using isolated sample data."""

import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pty
import select
import shlex
import shutil
import struct
import subprocess
import sys
import tempfile
import termios
import time
import tty

ROOT = Path(__file__).resolve().parents[1]
TITLE = "wraith documentation capture"


def provider():
    messages = json.loads(sys.stdin.read().split("Conversation JSON:\n", 1)[1])
    prompt = messages[-1]["content"]
    if "command result" in prompt or "background command" in prompt:
        answer = "the job is tracked in `/jobs`. keep working here while it finishes; its output stays available in `/history`."
    elif "run the local report" in prompt:
        command = "printf 'service report\\n443/tcp  open  https\\ncertificate: lab fixture\\n'; sleep 20; printf 'report complete\\n'"
        answer = "collect a local sample report without contacting a target.\n\n```wraith-run\n" + json.dumps({"command": command, "reason": "print the documentation fixture and demonstrate live background output"}) + "\n```"
    else:
        answer = "## inspect the certificate\n\ncheck **wraith::TARGET_DOMAIN** and record the negotiated protocol, issuer and expiration.\n\n```sh\nopenssl s_client -connect wraith::TARGET_DOMAIN:443 \\\n  -servername wraith::TARGET_DOMAIN -brief </dev/null\n```\n\n| observation | follow-up |\n| --- | --- |\n| expired certificate | verify renewal and deployment |\n| unexpected issuer | inspect the served chain |\n| successful handshake | compare against your tls policy |\n\nthis is a suggested command. open the block to edit it before running."
    print(json.dumps({"type": "stream_event", "event": {"delta": {"thinking": "start with a read-only certificate check. record evidence before drawing conclusions; a successful handshake does not prove the full configuration is correct."}}}), flush=True)
    for start in range(0, len(answer), 48):
        print(json.dumps({"type": "stream_event", "event": {"delta": {"text": answer[start:start + 48]}}}), flush=True)
        time.sleep(0.01)
    print(json.dumps({"type": "result", "usage": {"input_tokens": 1450, "output_tokens": 240}}), flush=True)


def drive(output, record):
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="wraith-capture-") as directory:
        workspace = Path(directory)
        for name in ("config", "data", "cache", "lab"):
            (workspace / name).mkdir()
        config = {
            "provider": "sample", "model": "documentation-fixture", "animations": False,
            "providers": {"sample": {"kind": "claude-cli", "model": "documentation-fixture",
                "models": ["documentation-fixture", "alternate-fixture"],
                "command": [sys.executable, str(Path(__file__).resolve()), "--provider"]}},
            "shell": ["bash", "--noprofile", "--norc", "-i"], "clipboard": [],
            "terminal": {"background": "theme"},
        }
        (workspace / "config/config.yaml").write_text(json.dumps(config))
        binary = ROOT / "bin/wraith"
        argv = [str(binary), "--config-dir", str(workspace / "config"),
                "--data-dir", str(workspace / "data"), "--cache-dir", str(workspace / "cache")]
        pid, master = pty.fork()
        if pid == 0:
            os.chdir(workspace / "lab")
            os.environ.update(TERM="xterm-256color", COLORTERM="truecolor", PS1="lab $ ")
            os.execv(argv[0], argv)
        size = shutil.get_terminal_size((120, 40))
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", size.lines, size.columns, 0, 0))
        stream = bytearray()
        scenes = []

        def pump(duration=0.5):
            deadline = time.monotonic() + duration
            while time.monotonic() < deadline:
                if select.select([master], [], [], min(0.05, max(0, deadline - time.monotonic())))[0]:
                    data = os.read(master, 65536)
                    if not data:
                        raise RuntimeError("wraith exited before capture finished")
                    stream.extend(data)
                    os.write(sys.stdout.fileno(), data)
                    if b"\x1b[6n" in data:
                        os.write(master, b"\x1b[1;1R")
                    if b"\x1b[c" in data:
                        os.write(master, b"\x1b[?1;2c")

        def send(keys, delay=0.4):
            os.write(master, keys.encode())
            pump(delay)

        def command(text):
            send(text + "\x13", 0.6)

        def capture(name, caption):
            pump(0.2)
            (output / (name + ".ansi")).write_bytes(stream)
            if not record:
                clients = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"]))
                windows = [c for c in clients if c.get("title") == TITLE]
                if len(windows) != 1:
                    raise RuntimeError("capture window is missing or ambiguous")
                window = windows[0]
                x, y = window["at"]
                w, h = window["size"]
                subprocess.run(["grim", "-g", f"{x},{y} {w}x{h}", str(output / (name + ".png"))], check=True)
            scenes.append({"file": name + (".ansi" if record else ".png"), "caption": caption})

        try:
            pump(2)
            capture("welcome", "new conversation; animated mark paused by configuration")
            command("/env set TARGET_DOMAIN=lab.example # isolated documentation target")
            send("\x1b")
            command("/pin scope: local documentation fixtures only")
            command("review the certificate on our lab service and list what to record")
            pump(1)
            capture("conversation", "sample response; markdown, table and locally resolved private variable")
            send("\t]za")
            capture("thinking", "expanded sample reasoning in the conversation")
            send("za\t")
            command("run the local report")
            pump(1)
            capture("approval", "approval for a real local printf and sleep command")
            send("\r", 1)
            command("/jobs")
            capture("jobs", "running background job with status and id")
            send("\r")
            capture("output", "live output from the local job")
            send("\x1b")
            for name, cmd, caption in [
                ("variables", "/env", "directory variables with attached comments"),
                ("pins", "/pins", "exact conversation facts"),
                ("context", "/context", "context usage and compaction policy"),
                ("models", "/models", "configured sample model catalog"),
                ("providers", "/providers", "configured provider picker"),
                ("settings", "/settings", "settings categories"),
                ("appearance", "/settings.appearance", "appearance settings"),
            ]:
                command(cmd)
                capture(name, caption)
                if name != "context":
                    send("\x1b")
            command("/settings.execution")
            send("?")
            capture("setting-help", "per-setting explanation")
            send("\x1b")
            send("\x1b")
            send("/settings.")
            capture("completion", "category completion after an exact command and dot")
            send("\x1b")
            send("ddi")
            send("\x1bs", 0.7)
            send("printf 'local shell\\n'; pwd\r")
            capture("side-shell", "independent bash pane alongside the conversation")
            send("\x1bs")
            send("\x1be", 0.7)
            send("printf 'scratch shell\\nready for edited commands\\n'\r")
            capture("scratch-shell", "centered bash pane")
            for _ in range(3):
                send("\x1b", 0.15)
            capture("terminal-copy", "terminal copy mode with vim cursor")
            send("i")
            send("\x1be")
            command("/resume")
            capture("sessions", "directory-scoped session picker")
            send("\x1b")
            command("/clear")
            for theme in ("phosphor", "rose-pine", "dracula", "everforest", "nord", "catppuccin-mocha", "tokyo", "ember", "ice", "mono"):
                command("/theme " + theme)
                capture("theme-" + theme, theme + " welcome screen")
            os.write(master, b"\x11")
            os.waitpid(pid, 0)
            pid = 0
        finally:
            if pid:
                os.kill(pid, 15)
                os.waitpid(pid, 0)
            os.close(master)
        manifest = {"binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                    "commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
                    "columns": size.columns, "rows": size.lines,
                    "content": "scripted local fixtures; no provider credentials or network requests",
                    "capture": "pty recording" if record else "grim capture of ghostty running wraith",
                    "scenes": scenes}
        (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=Path("/tmp/wraith-screenshots"))
    parser.add_argument("--record", action="store_true", help="record pty output without desktop capture")
    parser.add_argument("--drive", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--provider", action="store_true", help=argparse.SUPPRESS)
    args, _ = parser.parse_known_args()
    if args.provider:
        provider()
        return
    output = args.output.resolve()
    if args.drive or args.record:
        attributes = termios.tcgetattr(sys.stdin.fileno()) if sys.stdin.isatty() else None
        try:
            if attributes is not None:
                tty.setraw(sys.stdin.fileno())
            drive(output, args.record)
        finally:
            if attributes is not None:
                termios.tcsetattr(sys.stdin.fileno(), termios.TCSADRAIN, attributes)
        return
    for name in ("ghostty", "hyprctl", "grim"):
        if not shutil.which(name):
            parser.error(name + " is required for desktop capture")
    subprocess.run(["make", "build"], cwd=ROOT, check=True)
    output.mkdir(parents=True, exist_ok=True)
    command = ["ghostty", "--config-default-files=false", "--gtk-single-instance=false",
               "--title=" + TITLE, "--font-family=JetBrains Mono", "--font-size=12",
               "--background=#101419", "--background-opacity=1", "--window-decoration=false",
               "--window-padding-x=12", "--window-padding-y=12", "-e", sys.executable,
               str(Path(__file__).resolve()), "--drive", "--output", str(output)]
    subprocess.run(["hyprctl", "dispatch", "exec", "[float; size 1440 960; center] " + shlex.join(command)], check=True)
    print("capture window opened; keep it visible until it closes. files: " + str(output))


if __name__ == "__main__":
    main()
