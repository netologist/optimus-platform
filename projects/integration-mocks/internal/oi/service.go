package oi

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
)

// Register registers OI tools on an MCP server
func Register(s *mcpserver.Server) {
	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "oi.get_recent_events",
		Description: "Fetch real-time IoT and operational telemetry events for an asset",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","asset_id"]}`),
		Handler: func(args map[string]any) (any, error) {
			assetID, _ := args["asset_id"].(string)
			if assetID == "" {
				return nil, fmt.Errorf("asset_id is required")
			}
			return map[string]any{
				"asset_id": assetID,
				"telemetry": map[string]any{
					"temperature_celsius": 98.4,
					"vibration_rms":       4.2,
					"flow_rate_lpm":       142.0,
					"status":              "WARNING_CRITICAL_TEMPERATURE",
					"recorded_at":         time.Now().UTC().Format(time.RFC3339),
				},
			}, nil
		},
	})
}
