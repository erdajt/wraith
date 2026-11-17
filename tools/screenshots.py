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

from showcase import HEADERS, provider, walkthrough

ROOT = Path(__file__).resolve().parents[1]
TITLE = "wraith documentation capture"


def drive(output, record):
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="wraith-capture-") as directory:
        workspace = Path(directory)
        for name in ("config", "data", "cache", "lab"):
            (workspace / name).mkdir()
        config = {
            "provider": "sample", "model": "offline-showcase", "theme": "rose-pine", "animations": False,
            "editor": ["nvim", "-u", "NONE", "-n", "+set laststatus=0 noruler noshowmode"],
            "providers": {"sample": {"kind": "claude-cli", "model": "offline-showcase", "context": 32768,
                "models": ["offline-showcase", "alternate-fixture"],
                "command": [sys.executable, str(Path(__file__).resolve()), "--provider"]}},
            "shell": ["bash", "--noprofile", "--norc", "-i"], "clipboard": [],
            "terminal": {"background": "theme"},
        }
        (workspace / "config/config.yaml").write_text(json.dumps(config))
        (workspace / "lab/headers.txt").write_text(HEADERS)
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
            walkthrough(send, command, capture, pump)
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
    for name in ("ghostty", "hyprctl", "grim", "nvim"):
        if not shutil.which(name):
            parser.error(name + " is required for desktop capture")
    subprocess.run(["make", "build"], cwd=ROOT, check=True)
    output.mkdir(parents=True, exist_ok=True)
    command = ["ghostty", "--config-default-files=false", "--gtk-single-instance=false",
               "--title=" + TITLE, "--font-family=JetBrainsMono Nerd Font Mono", "--font-size=14",
               "--background=#101419", "--background-opacity=1", "--window-decoration=false",
               "--window-padding-x=0", "--window-padding-y=0", "--window-padding-balance=false", "-e", sys.executable,
               str(Path(__file__).resolve()), "--drive", "--output", str(output)]
    subprocess.run(["hyprctl", "dispatch", "exec", "[float; size 1440 900; center] " + shlex.join(command)], check=True)
    print("capture window opened; keep it visible until it closes. files: " + str(output))


if __name__ == "__main__":
    main()
