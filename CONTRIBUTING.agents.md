> For human contributors, see [`CONTRIBUTING.md`](CONTRIBUTING.md).

# Agent Contributor Guide for Focus

This guide is for coding agents (Claude Code, Codex, Kimi Code, OpenCode, Gemini CLI, etc.) that are asked to work on `focus`.

## Important: This Project Is Not Turnkey

`focus` is **not a plug-and-play demo**. Before you make any code change, you must verify and configure the environment you are running in. The project auto-detects a lot, but several critical pieces depend on the host machine and must be set up by you or the operator.

## Pre-Flight Checklist

Run through this checklist before editing any code:

- [ ] You are inside a real Git repository with at least one commit.
- [ ] You have read the human-facing [`CONTRIBUTING.md`](CONTRIBUTING.md).
- [ ] You have read the relevant docs in `docs/architecture/` and `docs/adr/` before touching core logic.
- [ ] You understand that `focus` creates and deletes Git worktrees under `.worktrees/` — do not run it against a repository you are not allowed to modify.

## 1. Agent Binaries Must Be Discoverable

`focus` launches external agents by looking for well-known binaries in `PATH`. The supported agents include:

- `claude` (Claude Code)
- `codex` (OpenAI Codex CLI)
- `kimi` (Kimi CLI)
- `opencode` (OpenCode CLI)
- `gemini` (Gemini CLI)

You or the operator must ensure the target binaries are installed and available in `PATH`. If an agent is installed but not found, check:

- Shell startup files (`.bashrc`, `.zshrc`, etc.) that modify `PATH`
- Symlinks in `~/.local/bin`
- Non-standard installation directories

## 2. Terminal Emulator Configuration

If the configuration has `agent.external_terminal: true`, `focus` will try to spawn a new terminal window for the agent session. This requires a supported terminal emulator.

Check `~/.config/focus/config.yaml`:

```yaml
agent:
  external_terminal: true      # or false
  terminal_emulator: kitty     # e.g. kitty, alacritty, wezterm, ghostty, iTerm2
```

If `terminal_emulator` is empty, `focus` attempts auto-detection. On an unsupported or misconfigured terminal, the agent session launch will fail. Set it explicitly or disable `external_terminal` if the host does not support external windows.

## 3. MCP and A2A Transport

`focus` uses MCP and A2A as shared-context protocols. Depending on the transport, you may need to configure sockets or ports.

Check `~/.config/focus/config.yaml`:

```yaml
agent:
  mcp_port: "127.0.0.1:0"      # TCP port for MCP
  mcp_socket: ""               # Unix-domain socket path for MCP (overrides mcp_port if set)
  a2a_socket: ""               # Unix-domain socket path for A2A
```

If the operator expects HTTP transport, leave `mcp_socket` empty and ensure `mcp_port` is reachable. If they expect Unix sockets, set absolute paths and verify the directory exists.

## 4. Default Providers

The default providers in `~/.config/focus/config.yaml` are hints for which agent to use for each task category:

```yaml
agent:
  research_provider: kimi
  architecture_provider: claude
  coding_provider: "codex,kimi,claude"
```

These names must match the agent binaries that are actually installed. If the named provider is missing, the launch will fail. Adjust them to match the available agents on this machine.

## 5. Editor Command

`focus` opens external editors for plan/task editing. The default is `nvim`.

```yaml
editor:
  command: nvim
```

Make sure the configured editor is installed and blocks until the file is closed (e.g. `code --wait` for VS Code). A non-blocking editor will cause race conditions.

## 6. Database Backend

By default `focus` uses an embedded SQLite database under `.focus/focus.db` in the target repository.

For PostgreSQL, the connection string must be supplied via environment variables or project-specific configuration. Do not assume PostgreSQL is available unless the operator explicitly says so.

## Build and Test Commands

Always run these after any change:

```bash
# Build the CLI
go build ./cmd/focus

# Run the short test suite (excludes long-running integration tests)
go test -short ./...

# Run all tests only when you have time and the full environment is ready
go test ./...
```

## Running Tests on Windows

On Windows, run `go test` from **Windows Terminal** or **PowerShell**, not
Git Bash / MSYS2 / MinGW shells. `go test` builds test binaries that import the
TUI library (`charm.land/bubbletea/v2`), whose package `init` probes the
console; under a pseudo-TTY (`tty: not a tty`, as Git Bash presents it) this
probe can stall and the test binary hangs on startup. The `focus` binary
itself is unaffected — this is a test-harness environment artifact, not a
code bug.

If a test suite appears to hang on Windows, switch to Windows Terminal and
re-run before investigating the test itself.

## Common Pitfalls

- **Do not assume the demo video or logo generation script works on your machine.** `scripts/generate_logo.py` looks for specific fonts and PIL.
- **Do not assume all tests pass on every OS.** If a test fails only on your platform, investigate whether the test is over-fitted to the author's environment (e.g. hard-coded `/home/rollingtherock/...` paths or Unix-specific PTY behavior).
- **Do not rewrite large subsystems in a single PR.** Keep changes small and focused.
- **Do not commit IDE-specific files.** They are already ignored in `.gitignore`.

## Where to Look

| Topic | Location |
|---|---|
| Architecture | `docs/architecture/` |
| Design decisions | `docs/adr/` |
| Release process | `docs/releases/` |
| Agent discovery | `internal/agents/` |
| Worktree management | `internal/worktree/` |
| Configuration | `internal/config/config.go` |
| Store / database | `internal/store/` |
| UI components | `internal/ui/` |
| Plugins | `internal/plugins/` |

## Need Help?

If the environment is not ready, stop and ask the operator to configure it. Do not patch around missing binaries or wrong configuration with brittle fallbacks unless the operator explicitly asks for a compatibility shim.
