# focus-tui

`focus-tui` is a terminal-native developer workspace: a step toward a developer operating system that runs inside the terminal.

It is not meant to be a wrapper for a single coding agent. The goal is to provide one structured workspace where `shell` sessions, coding agents, git workflows, and personal workflow components can coexist and stay visible together.

## Positioning

- Terminal-first, not GUI-first
- Native shell integration, not a launcher for external TUIs
- Agent-agnostic: works with any terminal-based coding agent, including `OpenCode`, `Claude Code`, and future CLI agents
- Personal workflow components such as `Todo` and `Pomodoro` remain first-class parts of the workspace instead of being treated as throwaway experiments

## Current Status

The project is currently transitioning from an earlier productivity-oriented TUI into a more general terminal developer workspace.

- `Phase 1` is largely complete: the embedded shell is now usable enough to support real terminal workflows
- `Phase 2` is the current focus: building a true multi-pane layout system
- Existing `Todo`, `Pomodoro`, header, footer, and local SQLite storage are still kept in the product and will continue to live alongside the shell-centric workflow

## What Exists Today

- Embedded shell powered by PTY + terminal emulation
- Keyboard-driven full-screen TUI built with `Bubble Tea`
- Header, footer, `Todo`, and `Pomodoro` components
- SQLite-backed local state for personal workflow features
- Overlay-based panel interactions that will be evolved into pane-based interactions

## Roadmap

### Phase 1: Embedded Shell Reliability

Make the built-in shell feel trustworthy enough for real work:

- PTY-backed shell execution
- Alternate screen handling
- Terminal capability forwarding
- Mouse forwarding and scrollback behavior
- Resize synchronization between terminal UI and PTY

### Phase 2: Multi-Pane Layout System

Build the core layout infrastructure required by everything that comes next:

- Multiple panes on screen at once
- Horizontal and vertical splits
- Pane focus routing
- Pane metadata such as `name`, `type`, `cwd`, and `status`
- Multi-instance shell panes
- Reuse of existing `Todo` and `Pomodoro` modules as normal panes

### Phase 3: Native Git and Worktree Workflow

Make daily git operations possible without leaving `focus-tui`:

- Repository status summary
- Worktree list and state
- Worktree create / switch / remove
- Worktree as the primary task container for shell, editor, review, and future agent workflows
- Overview page for orchestration plus full-screen worktree workspace pages for active development
- Diff views and staging workflow
- Commit flow inside the workspace

### Phase 4: Agent Session Visibility

Add structured visibility into agent activity without coupling the product to one vendor:

- Session history panes for supported agent tools
- Mapping sessions to `cwd`, branch, and worktree
- Surfacing which agent is active in which workspace
- Building toward multi-agent orchestration in one terminal-native environment

## Design Principles

- Native over glued-together integrations
- Structure over terminal window chaos
- Agent support without vendor lock-in
- Keep useful workflow tools instead of deleting them just because the product direction evolved
- Build the shell and pane system first, then layer git and agent-aware workflows on top

## Stack

- Go
- `Bubble Tea`
- `Lip Gloss`
- SQLite
- PTY + terminal emulation for embedded shell support

## Development Notes

- The current repository still contains code and docs from the earlier productivity-app phase
- The product direction is now broader: a terminal workspace that can host both development workflows and personal focus tools
- Near-term development is focused on the pane system rather than adding many new end-user features
