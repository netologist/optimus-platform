package plm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
)

// Register registers PLM tools on an MCP server
func Register(s *mcpserver.Server) {
	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "plm.search_documents",
		Description: "Perform hybrid semantic search (pgvector + keyword) on technical engineering and service manuals",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","query"]}`),
		Handler: func(args map[string]any) (any, error) {
			query, _ := args["query"].(string)
			if query == "" {
				return nil, fmt.Errorf("query is required")
			}

			// Simulates retrieval of PLM-COOL-4021 §4.2
			matches := []map[string]any{}
			if strings.Contains(strings.ToLower(query), "overheating") || strings.Contains(strings.ToLower(query), "cooling") || strings.Contains(strings.ToLower(query), "p-104") {
				matches = append(matches, map[string]any{
					"document_id": "PLM-COOL-4021",
					"title":       "Centrifugal Pump Cooling Loop Maintenance Manual",
					"section":     "§4.2",
					"content":     "Recurrent overheating in pump cooling manifolds typically indicates thermostat failure (part SP-COOL-9981). Replace thermostat and flush secondary cooling line immediately.",
					"score":       0.92,
					"spare_part":  "SP-COOL-9981",
				})
			}

			return map[string]any{
				"query":   query,
				"count":   len(matches),
				"results": matches,
			}, nil
		},
	})
}
