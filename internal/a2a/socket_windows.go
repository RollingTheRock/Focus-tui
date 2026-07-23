//go:build windows

package a2a

import (
	"fmt"
	"net"
)

// listenSocket on Windows returns an error because Unix domain sockets
// are not fully supported. A2A communication on Windows should use
// TCP-based transport instead.
func listenSocket(socketPath string) (net.Listener, error) {
	if socketPath == "" {
		return nil, fmt.Errorf("socket path required")
	}
	return nil, fmt.Errorf("unix domain sockets not supported on Windows; use TCP transport")
}
