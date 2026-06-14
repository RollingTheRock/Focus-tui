# Recording Agents for README Demos

## The Problem

`focus` can launch real coding agents in external terminal emulators (`kitty`,
`alacritty`, `wezterm`, `gnome-terminal`, ...). Tools like
[VHS](https://github.com/charmbracelet/vhs) record the terminal they start, so
any window that pops out is invisible to the recording.

## Solutions

### 1. Keep the agent inside the recording window

Set `agent.external_terminal: false` in `~/.config/focus/config.yaml`, then run
the agent inside `focus`'s embedded shell pane. Alternatively, use a terminal
multiplexer (`tmux`, `screen`, `zellij`) with `focus` in one pane and the agent
in another pane of the same window.

The local README demo (`scripts/record-demo-local.tape`) uses this approach
with a mock `claude` binary placed first in `PATH`, so no API key is required
and the whole workflow stays inside one terminal window.

### 2. Capture the external window separately and composite

- Record `focus` with VHS as usual.
- Record the popped-out agent terminal with `ffmpeg x11grab` (X11) or OBS.
- Crop and overlay the two recordings in an editor.

Example with `ffmpeg` on X11:

```bash
# Find the window ID / geometry of the agent terminal
xwininfo

# Record a 1920x1080 region
ffmpeg -f x11grab -r 30 -s 1920x1080 -i :0.0+0,0 -c:v libx264 agent.mp4
```

Composite side-by-side:

```bash
ffmpeg -i focus.mp4 -i agent.mp4 -filter_complex hstack demo.mp4
```

### 3. Use a tiling terminal emulator

Run `focus` in one pane of `kitty`, `wezterm`, or `tmux` and the agent in
another pane of the same window. VHS captures the whole window, so both are
visible.

## Recommended workflow for the focus README

We use option 1 because it is fully reproducible, requires no API keys, and
keeps the entire workflow in a single GIF.
