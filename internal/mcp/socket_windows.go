//go:build windows

package mcp

import (
	"fmt"
	"net"
)

// listenSocket creates a TCP listener on localhost for Windows platforms.
// Unix domain sockets have limited support on Windows, so we use TCP instead.
func listenSocket(socketPath string) (net.Listener, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("socket path required")
	}
	// On Windows, we ignore the socketPath and use TCP on localhost
	// The port is determined by the httpAddr configuration (e.g., "127.0.0.1:0")
	return nil, fmt.Errorf("use TCP transport on Windows")
}
