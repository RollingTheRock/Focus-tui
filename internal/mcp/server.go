package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ToolHandler func(params map[string]any) (map[string]any, error)

type Server struct {
	socketPath string

	mu       sync.RWMutex
	running  bool
	tools    map[string]ToolHandler
	listener net.Listener
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type rpcResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *rpcError      `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		tools:      make(map[string]ToolHandler),
		conns:      make(map[net.Conn]struct{}),
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.socketPath == "" {
		return fmt.Errorf("mcp socket path required")
	}
	if s.running {
		return nil
	}

	listener, err := listenUnixSocket(s.socketPath)
	if err != nil {
		return err
	}

	s.listener = listener
	s.running = true
	s.wg.Add(1)
	go s.acceptLoop(listener)
	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	listener := s.listener
	s.listener = nil
	conns := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.conns = make(map[net.Conn]struct{})
	s.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	s.wg.Wait()

	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
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

func (s *Server) acceptLoop(listener net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()
			if !running {
				return
			}
			continue
		}
		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
	}()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	for {
		var req rpcRequest
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			return
		}
		resp := s.handleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

func (s *Server) handleRequest(req rpcRequest) rpcResponse {
	resp := rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		resp.Error = &rpcError{
			Code:    -32600,
			Message: "method required",
		}
		return resp
	}
	result, err := s.CallTool(method, req.Params)
	if err != nil {
		resp.Error = &rpcError{
			Code:    -32000,
			Message: err.Error(),
		}
		return resp
	}
	if result == nil {
		result = map[string]any{}
	}
	resp.Result = result
	return resp
}

func listenUnixSocket(socketPath string) (net.Listener, error) {
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
