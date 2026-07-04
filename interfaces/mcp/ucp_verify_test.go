package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPToolsList_ViaRouter(t *testing.T) {
	router := NewMCPRouter()
	router.Register(NewCatalogMCPHandler(nil))

	body := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  *struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON-RPC response: %v\nbody: %s", err, w.Body.String())
	}

	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Error.Message)
	}
	if result.Result == nil {
		t.Fatal("expected result field")
	}
	if len(result.Result.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	names := make(map[string]bool)
	for _, tool := range result.Result.Tools {
		if tool.Name == "" {
			t.Error("tool with empty name")
		}
		if names[tool.Name] {
			t.Errorf("duplicate tool name: %s", tool.Name)
		}
		names[tool.Name] = true
	}
}

func TestMCPToolsList_CatalogPreRegistered(t *testing.T) {
	router := NewMCPRouter()
	router.Register(NewCatalogMCPHandler(nil))

	body := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if !strings.Contains(w.Body.String(), `search_catalog`) {
		t.Error("expected search_catalog in tools list")
	}
	if !strings.Contains(w.Body.String(), `lookup_catalog`) {
		t.Error("expected lookup_catalog in tools list")
	}
	if !strings.Contains(w.Body.String(), `get_product`) {
		t.Error("expected get_product in tools list")
	}
}
