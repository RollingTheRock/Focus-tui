package trellis

import "errors"

var (
	// ErrTrellisNotInstalled is returned when the trellis CLI is not found in PATH.
	ErrTrellisNotInstalled = errors.New("trellis CLI not installed; run: npm install -g @mindfoldhq/trellis")

	// ErrPythonNotFound is returned when no suitable Python 3.9+ interpreter is found.
	ErrPythonNotFound = errors.New("python 3.9+ not found; please install Python")

	// ErrTrellisInitFailed is returned when trellis init fails.
	ErrTrellisInitFailed = errors.New("trellis init failed")

	// ErrTaskNotSynced is returned when a Focus task has not been synced to Trellis yet.
	ErrTaskNotSynced = errors.New("task not synced to trellis")

	// ErrContextBuildFailed is returned when building agent context fails.
	ErrContextBuildFailed = errors.New("failed to build agent context")
)
