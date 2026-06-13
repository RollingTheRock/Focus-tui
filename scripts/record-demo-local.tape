# focus README full-workflow demo — run locally without API keys.
#
# Why this script exists:
#   focus launches real coding agents. By default (agent.external_terminal=false)
#   the UI would run the agent in the embedded shell pane; in the current
#   single-page design the built-in agent launcher is disabled and agents are
#   opened in external terminal emulators. External windows cannot be captured by
#   VHS, so this demo manually runs a mock `claude` binary inside the embedded
#   shell pane. That keeps everything inside the same terminal window and makes
#   the full workflow (DAG -> worktree -> agent -> Agent Store) recordable.
#
# How to record agents that open in external windows:
#   1. Keep them inside one window for the demo:
#      - Set agent.external_terminal=false in ~/.config/focus/config.yaml.
#      - Use the embedded shell pane or a terminal multiplexer (tmux/screen/zellij)
#        and run the agent command manually, as this tape does.
#   2. Capture external windows separately and composite:
#      - Record focus with VHS as usual.
#      - Record the external terminal with ffmpeg x11grab/Wayland or OBS.
#      - Crop and overlay the two videos in an editor, or stitch side-by-side
#        with ffmpeg hstack/vstack.
#   3. Use a tiling terminal emulator:
#      - kitty/wezterm/tmux can split the same window; run focus in one pane and
#        the agent in another. VHS captures the whole window.
#
# Prereqs: brew install vhs ttyd ffmpeg chafa; JetBrainsMono Nerd Font.
# Run with: PATH="$(pwd)/scripts:$PATH" vhs scripts/record-demo-local.tape
# Ensure ~/.config/focus/config.yaml has agent.external_terminal set to false.

Output docs/assets/focus-demo.gif

Set Shell "bash"
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set Width 1920
Set Height 1080
Set Theme "Dracula"
Set Padding 0
Set Margin 30
Set MarginFill "#1e1e2e"
Set BorderRadius 12
Set WindowBar Colorful
Set TypingSpeed 35ms

Hide
Type "go build -o focus ./cmd/focus && scripts/prepare-demo-data.sh"
Enter
Sleep 6s
Type "clear"
Enter
Show

Type "./focus"
Enter
Sleep 3s

# --- Help overlay -----------------------------------------------------------
Type "?"
Sleep 1s
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "q"
Sleep 800ms

# --- DAG: expand the phase and show its steps -------------------------------
Type "z"
Sleep 1s
Type "q"
Sleep 500ms

# --- Move focus to the shell pane (DAG -> Worktree -> Detail -> Shell) ------
Ctrl+L
Sleep 400ms
Ctrl+J
Sleep 400ms
Ctrl+L
Sleep 400ms

# Enter shell-input mode, clear startup noise, and run the mock agent.
Enter
Sleep 400ms
Type "clear"
Enter
Sleep 500ms
Type "FOCUS_TRELLIS_CONTEXT=demo-phase-auth claude"
Sleep 500ms
Enter
Sleep 5s

# Leave shell-input mode so global shortcuts work again.
Escape
Sleep 500ms

# --- Global overview + Agent Store ------------------------------------------
Ctrl+G
Sleep 1s
Type "S"
Sleep 2s
Type "j"
Sleep 400ms
Type "j"
Sleep 400ms
Type "j"
Sleep 400ms
Type "q"
Sleep 800ms

Ctrl+G
Sleep 1s
Type "q"
Sleep 1s
