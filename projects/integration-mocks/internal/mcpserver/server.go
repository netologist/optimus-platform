package mcpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// ToolDefinition defines a tool exposed via MCP
type ToolDefinition struct {
	Name        string                                 `json:"name"`
	Description string                                 `json:"description"`
	InputSchema json.RawMessage                        `json:"inputSchema"`
	Handler     func(args map[string]any) (any, error) `json:"-"`
}

// CallRecord tracks tool invocations for testing and auditing
type CallRecord struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// Server provides an MCP HTTP server
type Server struct {
	systemName string
	tools      map[string]ToolDefinition
	callLog    []CallRecord
	mu         sync.RWMutex
}

func New(systemName string) *Server {
	return &Server{
		systemName: systemName,
		tools:      make(map[string]ToolDefinition),
		callLog:    make([]CallRecord, 0),
	}
}

func (s *Server) RegisterTool(tool ToolDefinition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
}

func (s *Server) GetCallLog() []CallRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]CallRecord, len(s.callLog))
	copy(res, s.callLog)
	return res
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Support call log inspection endpoint for E2E testing
	if r.URL.Path == "/call-log" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.GetCallLog())
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "tools/list":
		s.mu.RLock()
		toolsList := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			toolsList = append(toolsList, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
		s.mu.RUnlock()
		resp.Result = map[string]any{"tools": toolsList}

	case "tools/call":
		var params callToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = map[string]any{"code": -32602, "message": "Invalid params"}
			break
		}

		s.mu.Lock()
		tool, exists := s.tools[params.Name]
		if !exists {
			s.mu.Unlock()
			resp.Error = map[string]any{"code": -32601, "message": fmt.Sprintf("Tool %s not found", params.Name)}
			break
		}

		s.callLog = append(s.callLog, CallRecord{
			Tool: params.Name,
			Args: params.Arguments,
		})
		s.mu.Unlock()

		res, err := tool.Handler(params.Arguments)
		if err != nil {
			resp.Error = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			resp.Result = map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": fmt.Sprintf("%v", res),
					},
				},
				"data": res,
			}
		}

	default:
		resp.Error = map[string]any{"code": -32601, "message": "Method not found"}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
