package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ToolHandler is the function signature for an MCP tool implementation.
type ToolHandler func(params map[string]any) (map[string]any, error)

// ToolMeta holds metadata and handler for a registered tool.
type ToolMeta struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

// Server implements a standard Model Context Protocol server.
type Server struct {
	mu sync.RWMutex

	// Tool registry.
	tools map[string]ToolMeta

	// Unix socket transport (optional, for internal/local connections).
	unixSocketPath string
	unixListener   net.Listener
	unixRunning    bool

	// HTTP transport.
	httpAddr     string
	httpListener net.Listener
	httpServer   *http.Server
	httpRunning  bool
	httpURL      string

	// Lifecycle.
	wg sync.WaitGroup
}

// NewServer creates a new MCP server.
// unixSocketPath may be empty to disable Unix socket transport.
// httpAddr may be empty to disable HTTP transport (e.g. ":0" for auto port).
func NewServer(unixSocketPath, httpAddr string) *Server {
	return &Server{
		unixSocketPath: unixSocketPath,
		httpAddr:       httpAddr,
		tools:          make(map[string]ToolMeta),
	}
}

// ── Tool registration ──

// RegisterTool registers a tool with the server.
func (s *Server) RegisterTool(name, description string, schema map[string]any, handler ToolHandler) error {
	if name == "" {
		return fmt.Errorf("tool name required")
	}
	if handler == nil {
		return fmt.Errorf("tool handler required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = ToolMeta{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Handler:     handler,
	}
	return nil
}

// ToolCount returns the number of registered tools.
func (s *Server) ToolCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tools)
}

// ── Start / Stop ──

// Start starts the Unix socket transport if configured.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unixSocketPath == "" {
		return nil
	}
	if s.unixRunning {
		return nil
	}
	listener, err := listenUnixSocket(s.unixSocketPath)
	if err != nil {
		return fmt.Errorf("mcp unix socket listen: %w", err)
	}
	s.unixListener = listener
	s.unixRunning = true
	s.wg.Add(1)
	go s.acceptLoop(listener)
	return nil
}

// StartHTTP starts the HTTP transport if configured.
// Returns the actual URL (e.g. "http://127.0.0.1:18765/mcp").
func (s *Server) StartHTTP() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpAddr == "" {
		return "", nil
	}
	if s.httpRunning {
		return s.httpURL, nil
	}
	listener, err := net.Listen("tcp", s.httpAddr)
	if err != nil {
		return "", fmt.Errorf("mcp http listen: %w", err)
	}
	s.httpListener = listener
	s.httpURL = fmt.Sprintf("http://%s/mcp", listener.Addr().String())

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleHTTP)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})

	s.httpServer = &http.Server{
		Handler:     mux,
		ReadTimeout: 30 * time.Second,
	}
	s.httpRunning = true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		_ = s.httpServer.Serve(listener)
	}()
	return s.httpURL, nil
}

// Stop gracefully shuts down all transports.
func (s *Server) Stop() error {
	s.mu.Lock()
	unixListener := s.unixListener
	s.unixListener = nil
	s.unixRunning = false
	httpServer := s.httpServer
	s.httpServer = nil
	s.httpRunning = false
	s.mu.Unlock()

	var firstErr error
	if unixListener != nil {
		if err := unixListener.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := os.Remove(s.unixSocketPath); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.wg.Wait()
	return firstErr
}

// Running reports whether the Unix socket transport is running.
func (s *Server) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unixRunning
}

// HTTPRunning reports whether the HTTP transport is running.
func (s *Server) HTTPRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.httpRunning
}

// HTTPURL returns the HTTP endpoint URL.
func (s *Server) HTTPURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.httpURL
}

// SocketPath returns the Unix socket path.
func (s *Server) SocketPath() string {
	return s.unixSocketPath
}

// ── Unix socket accept loop ──

func (s *Server) acceptLoop(listener net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			s.mu.RLock()
			running := s.unixRunning
			s.mu.RUnlock()
			if !running {
				return
			}
			continue
		}
		s.mu.RLock()
		if !s.unixRunning {
			s.mu.RUnlock()
			_ = conn.Close()
			return
		}
		s.mu.RUnlock()
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer func() { _ = conn.Close() }()
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	for {
		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			return
		}
		resp := s.handleJSONRPC(req)
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

// ── HTTP handler ──

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"only POST supported"}}`, http.StatusMethodNotAllowed)
		return
	}
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCError(w, nil, ErrParseError, "parse error")
		return
	}
	resp := s.handleJSONRPC(req)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ── JSON-RPC dispatch ──

func (s *Server) handleJSONRPC(req JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		resp.Error = &JSONRPCError{Code: ErrInvalidRequest, Message: "method required"}
		return resp
	}

	switch method {
	case "initialize":
		resp.Result = InitializeResult{
			ProtocolVersion: ProtocolVersion,
			Capabilities: ServerCapabilities{
				Tools: &ToolCapabilities{ListChanged: true},
			},
			ServerInfo: Implementation{Name: "focus-tui", Version: "1.0"},
		}
	case "notifications/initialized":
		// No response required for notifications.
		return JSONRPCResponse{JSONRPC: "2.0"}
	case "tools/list":
		resp.Result = s.listTools()
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			resp.Error = &JSONRPCError{Code: ErrInternalError, Message: err.Error()}
		} else {
			resp.Result = result
		}
	case "ping":
		resp.Result = map[string]any{}
	default:
		resp.Error = &JSONRPCError{Code: ErrMethodNotFound, Message: fmt.Sprintf("method %q not found", method)}
	}
	return resp
}

func (s *Server) listTools() ListToolsResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tools := make([]Tool, 0, len(s.tools))
	for _, meta := range s.tools {
		schema := meta.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		tools = append(tools, Tool{
			Name:        meta.Name,
			Description: meta.Description,
			InputSchema: schema,
		})
	}
	return ListToolsResult{Tools: tools}
}

func (s *Server) callTool(params map[string]any) (CallToolResult, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return CallToolResult{}, fmt.Errorf("tool name required")
	}
	s.mu.RLock()
	meta, ok := s.tools[name]
	s.mu.RUnlock()
	if !ok {
		return CallToolResult{}, fmt.Errorf("tool %q not found", name)
	}

	args, _ := params["arguments"].(map[string]any)
	if args == nil {
		args = map[string]any{}
	}

	result, err := meta.Handler(args)
	if err != nil {
		text := fmt.Sprintf("Error: %v", err)
		return CallToolResult{
			Content: []Content{NewTextContent(text)},
			IsError: true,
		}, nil
	}

	text := toolResultToText(result)
	return CallToolResult{
		Content: []Content{NewTextContent(text)},
	}, nil
}

// toolResultToText serialises a handler result into a human-readable string.
func toolResultToText(result map[string]any) string {
	if result == nil {
		return "{}"
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(b)
}

func writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	})
}

// listenUnixSocket creates a Unix domain socket listener, cleaning up stale sockets.
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
