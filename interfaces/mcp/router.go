package mcp

import (
	"context"
	"encoding/json"
	"net/http"
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

type MCPRouter struct {
	tools     []ToolDefinition
	providers map[string]ToolProvider
}

func NewMCPRouter() *MCPRouter {
	return &MCPRouter{
		providers: make(map[string]ToolProvider),
	}
}

func (r *MCPRouter) Register(provider ToolProvider) {
	tools := provider.ListTools()
	r.tools = append(r.tools, tools...)
	for _, t := range tools {
		r.providers[t.Name] = provider
	}
}

func (r *MCPRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
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
