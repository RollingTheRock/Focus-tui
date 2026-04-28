package mcp

import (
	"fmt"
	"sync"
)

type ToolHandler func(params map[string]any) (map[string]any, error)

type Server struct {
	socketPath string

	mu      sync.RWMutex
	running bool
	tools   map[string]ToolHandler
}

func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		tools:      make(map[string]ToolHandler),
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.socketPath == "" {
		return fmt.Errorf("mcp socket path required")
	}
	s.running = true
	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	return nil
}

func (s *Server) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Server) SocketPath() string {
	return s.socketPath
}

func (s *Server) RegisterTool(name string, handler ToolHandler) error {
	if name == "" {
		return fmt.Errorf("tool name required")
	}
	if handler == nil {
		return fmt.Errorf("tool handler required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = handler
	return nil
}

func (s *Server) CallTool(name string, params map[string]any) (map[string]any, error) {
	s.mu.RLock()
	running := s.running
	handler := s.tools[name]
	s.mu.RUnlock()

	if !running {
		return nil, fmt.Errorf("mcp server not running")
	}
	if handler == nil {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return handler(params)
}
