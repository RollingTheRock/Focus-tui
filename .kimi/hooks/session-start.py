#!/usr/bin/env python3
"""Kimi SessionStart hook — injects Trellis context on session start.

This hook mirrors Claude Code's session-start.py for Kimi Code CLI.
It reads .trellis/ state and outputs context for the Kimi session.
"""

import json
import os
import subprocess
import sys


def main():
    repo_root = os.environ.get("KIMI_WORKSPACE", os.getcwd())
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
            # Kimi hooks can output plain text that gets injected.
            print(result.stdout)


if __name__ == "__main__":
    main()
