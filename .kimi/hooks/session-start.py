#!/usr/bin/env python3
"""Kimi SessionStart hook — injects Trellis context on session start.

Kimi Code CLI passes a JSON object via stdin with fields like:
  {"session_id": "...", "cwd": "/path/to/project",
   "hook_event_name": "SessionStart", "source": "startup"}

The script reads the current Trellis state and prints context to stdout,
which Kimi CLI injects into the session context.
"""

import json
import os
import subprocess
import sys


def main():
    # Read hook context from stdin.
    try:
        hook_ctx = json.load(sys.stdin)
    except json.JSONDecodeError:
        hook_ctx = {}

    # Determine repo root: prefer cwd from hook context, then env, then os.getcwd().
    repo_root = hook_ctx.get("cwd", "")
    if not repo_root:
        repo_root = os.environ.get("KIMI_WORKSPACE", "")
    if not repo_root:
        repo_root = os.getcwd()

    trellis_dir = os.path.join(repo_root, ".trellis")
    if not os.path.isdir(trellis_dir):
        return

    # Run get_context.py to get the full Trellis context.
    get_ctx = os.path.join(trellis_dir, "scripts", "get_context.py")
    if os.path.exists(get_ctx):
        result = subprocess.run(
            [sys.executable, get_ctx],
            capture_output=True,
            text=True,
            cwd=repo_root,
        )
        if result.returncode == 0 and result.stdout:
            print(result.stdout)


if __name__ == "__main__":
    main()
