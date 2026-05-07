package mcp

// JSONRPCRequest is a standard JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// JSONRPCResponse is a standard JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
	Error   *JSONRPCError  `json:"error,omitempty"`
}

// JSONRPCError is a standard JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error codes per MCP spec.
const (
	ErrParseError     = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternalError  = -32603
)

// ProtocolVersion is the MCP protocol version we support.
const ProtocolVersion = "2024-11-05"

// ── Initialize ──

// InitializeRequest is sent by the client to begin a session.
type InitializeRequest struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// InitializeResult is returned by the server.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

// ClientCapabilities describes what the client supports.
type ClientCapabilities struct {
	// Empty for now; can be extended.
}

// ServerCapabilities describes what the server supports.
type ServerCapabilities struct {
	Tools     *ToolCapabilities     `json:"tools,omitempty"`
	Resources *ResourceCapabilities `json:"resources,omitempty"`
}

// ToolCapabilities describes tool-related capabilities.
type ToolCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapabilities describes resource-related capabilities.
type ResourceCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
	Subscribe   bool `json:"subscribe,omitempty"`
}

// Implementation identifies a client or server.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ── Tools ──

// Tool describes a callable tool exposed by the server.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ListToolsResult is the response to tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolRequest is the request body for tools/call.
type CallToolRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// CallToolResult is the response to tools/call.
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content is a union type for tool result content.
type Content interface {
	contentType() string
}

// TextContent is text returned by a tool.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (TextContent) contentType() string { return "text" }

// ImageContent is an image returned by a tool.
type ImageContent struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func (ImageContent) contentType() string { return "image" }

// NewTextContent creates a TextContent wrapper.
func NewTextContent(text string) Content {
	return TextContent{Type: "text", Text: text}
}

// ── Resources ──

// Resource describes a readable resource exposed by the server.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent is the content of a resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ResourceHandler reads a resource by URI.
type ResourceHandler func(uri string) (ResourceContent, error)

// ResourceMeta holds metadata and handler.
type ResourceMeta struct {
	Name        string
	Description string
	MimeType    string
	Handler     ResourceHandler
}

// ListResourcesResult is the response to resources/list.
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// ReadResourceResult is the response to resources/read.
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}
