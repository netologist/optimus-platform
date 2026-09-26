package eam

import (
	"encoding/json"
	"fmt"

	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
)

// RegisterRegisters registers EAM tools on an MCP server
func Register(s *mcpserver.Server) {
	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "eam.get_asset",
		Description: "Retrieve asset master record by asset ID and tenant",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","asset_id"]}`),
		Handler: func(args map[string]any) (any, error) {
			assetID, _ := args["asset_id"].(string)
			if assetID == "" {
				return nil, fmt.Errorf("asset_id is required")
			}
			return map[string]any{
				"asset_id":    assetID,
				"name":        "Pump " + assetID,
				"type":        "Centrifugal Pump",
				"plant":       "Manchester",
				"status":      "OPERATIONAL",
				"criticality": "HIGH",
			}, nil
		},
	})

	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "eam.get_maintenance_history",
		Description: "Retrieve recent maintenance events and failures for an asset",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","asset_id"]}`),
		Handler: func(args map[string]any) (any, error) {
			assetID, _ := args["asset_id"].(string)
			if assetID == "" {
				return nil, fmt.Errorf("asset_id is required")
			}
			// Realistic fixture data for P-104 overheating scenario
			return map[string]any{
				"asset_id":              assetID,
				"failures_last_30_days": 4,
				"events": []map[string]any{
					{"date": "2026-08-30", "event": "overheating", "resolved": true},
					{"date": "2026-09-06", "event": "overheating", "resolved": true},
					{"date": "2026-09-14", "event": "overheating", "resolved": true},
					{"date": "2026-09-22", "event": "overheating", "resolved": false},
				},
			}, nil
		},
	})
}
