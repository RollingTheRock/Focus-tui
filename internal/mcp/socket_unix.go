//go:build !windows

package mcp

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// listenSocket creates a Unix domain socket listener on non-Windows platforms,
// cleaning up stale sockets.
func listenSocket(socketPath string) (net.Listener, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("socket path required")
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", socketPath)
	if err == nil {
		return listener, nil
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		return nil, err
	}
	conn, dialErr := net.DialTimeout("unix", socketPath, 250*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("socket already in use: %s", socketPath)
	}
	if removeErr := os.Remove(socketPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return nil, removeErr
	}
	return net.Listen("unix", socketPath)
}
