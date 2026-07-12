package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
	ID      any       `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties"`
}

type PropertySchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolProvider interface {
	ListTools() []ToolDefinition
	HandleTool(ctx context.Context, name string, params json.RawMessage) (any, error)
}

type sseClient struct {
	ch     chan []byte
	done   chan struct{}
	closed bool
	mu     sync.Mutex
}

func (c *sseClient) send(data []byte) bool {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	select {
	case c.ch <- data:
	case <-c.done:
		c.closed = true
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()
	return true
}

func (c *sseClient) close() {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		close(c.done)
	}
	c.mu.Unlock()
}

type MCPRouter struct {
	mu        sync.Mutex
	tools     []ToolDefinition
	providers map[string]ToolProvider
	sessions  map[string]*sseClient
}

func NewMCPRouter() *MCPRouter {
	return &MCPRouter{
		providers: make(map[string]ToolProvider),
		sessions:  make(map[string]*sseClient),
	}
}

func (r *MCPRouter) Register(provider ToolProvider) {
	tools := provider.ListTools()
	r.tools = append(r.tools, tools...)
	for _, t := range tools {
		r.providers[t.Name] = provider
	}
}

func (r *MCPRouter) generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (r *MCPRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet && req.URL.Path == "/mcp" {
		r.handleSSE(w, req)
		return
	}

	if req.Method == http.MethodPost && len(req.URL.Path) > 5 && req.URL.Path[:5] == "/mcp/" {
		r.handleSessionMessage(w, req)
		return
	}

	if req.Method != http.MethodPost {
		writeRPCError(w, nil, -32600, "only POST is accepted")
		return
	}

	var rpcReq jsonRPCRequest
	if err := json.NewDecoder(req.Body).Decode(&rpcReq); err != nil {
		writeRPCError(w, nil, -32700, "parse error: invalid JSON")
		return
	}

	if rpcReq.JSONRPC != "2.0" {
		writeRPCError(w, rpcReq.ID, -32600, "invalid jsonrpc version")
		return
	}

	switch rpcReq.Method {
	case "tools/list":
		r.handleListTools(w, rpcReq)
	case "tools/call":
		r.handleCallTool(w, req, rpcReq)
	default:
		writeRPCError(w, rpcReq.ID, -32601, "method not found: "+rpcReq.Method)
	}
}

func (r *MCPRouter) handleSSE(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sessionID := r.generateSessionID()
	client := &sseClient{
		ch:   make(chan []byte, 64),
		done: make(chan struct{}),
	}

	r.mu.Lock()
	r.sessions[sessionID] = client
	r.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	_, _ = fmt.Fprintf(w, "event: session_id\ndata: %s\n\n", sessionID)
	flusher.Flush()

	ctx := req.Context()
	for {
		select {
		case <-ctx.Done():
			r.mu.Lock()
			delete(r.sessions, sessionID)
			r.mu.Unlock()
			client.close()
			return
		case data, ok := <-client.ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (r *MCPRouter) handleSessionMessage(w http.ResponseWriter, req *http.Request) {
	sessionID := req.URL.Path[5:]

	r.mu.Lock()
	client, ok := r.sessions[sessionID]
	r.mu.Unlock()

	if !ok {
		http.Error(w, "invalid session", http.StatusNotFound)
		return
	}

	var rpcReq jsonRPCRequest
	if err := json.NewDecoder(req.Body).Decode(&rpcReq); err != nil {
		resp, _ := json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
			ID:      nil,
		})
		client.send(resp)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	go func() {
		switch rpcReq.Method {
		case "tools/list":
			r.handleListToolsSSE(client, rpcReq)
		case "tools/call":
			r.handleCallToolSSE(client, req, rpcReq)
		default:
			resp, _ := json.Marshal(jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32601, Message: "method not found: " + rpcReq.Method},
				ID:      rpcReq.ID,
			})
			client.send(resp)
		}
	}()
}

func (r *MCPRouter) handleListToolsSSE(client *sseClient, req jsonRPCRequest) {
	resp, _ := json.Marshal(jsonRPCResponse{
		JSONRPC: "2.0",
		Result:  map[string]any{"tools": r.tools},
		ID:      req.ID,
	})
	client.send(resp)
}

func (r *MCPRouter) handleCallToolSSE(client *sseClient, httpReq *http.Request, rpcReq jsonRPCRequest) {
	var params toolCallParams
	if err := json.Unmarshal(rpcReq.Params, &params); err != nil {
		resp, _ := json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32602, Message: "invalid params"},
			ID:      rpcReq.ID,
		})
		client.send(resp)
		return
	}

	provider, ok := r.providers[params.Name]
	if !ok {
		resp, _ := json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32601, Message: "tool not found: " + params.Name},
			ID:      rpcReq.ID,
		})
		client.send(resp)
		return
	}

	result, err := provider.HandleTool(httpReq.Context(), params.Name, params.Arguments)
	if err != nil {
		resp, _ := json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32000, Message: err.Error()},
			ID:      rpcReq.ID,
		})
		client.send(resp)
		return
	}

	data, _ := json.Marshal(result)
	resp, _ := json.Marshal(jsonRPCResponse{
		JSONRPC: "2.0",
		Result: map[string]any{
			"content": []toolContent{
				{Type: "text", Text: string(data)},
			},
		},
		ID: rpcReq.ID,
	})
	client.send(resp)
}

func (r *MCPRouter) handleListTools(w http.ResponseWriter, req jsonRPCRequest) {
	writeRPCResult(w, req.ID, map[string]any{
		"tools": r.tools,
	})
}

func (r *MCPRouter) handleCallTool(w http.ResponseWriter, req *http.Request, rpcReq jsonRPCRequest) {
	var params toolCallParams
	if err := json.Unmarshal(rpcReq.Params, &params); err != nil {
		writeRPCError(w, rpcReq.ID, -32602, "invalid params")
		return
	}

	provider, ok := r.providers[params.Name]
	if !ok {
		writeRPCError(w, rpcReq.ID, -32601, "tool not found: "+params.Name)
		return
	}

	result, err := provider.HandleTool(req.Context(), params.Name, params.Arguments)
	if err != nil {
		writeRPCError(w, rpcReq.ID, -32000, err.Error())
		return
	}

	data, _ := json.Marshal(result)
	writeRPCResult(w, rpcReq.ID, map[string]any{
		"content": []toolContent{
			{Type: "text", Text: string(data)},
		},
	})
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func writeRPCError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		Error: &rpcError{
			Code:    code,
			Message: msg,
		},
		ID: id,
	})
}
