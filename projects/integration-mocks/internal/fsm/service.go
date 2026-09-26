package fsm

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
)

var (
	workOrdersMu sync.Mutex
	workOrders   = make(map[string]map[string]any)
)

// Register registers FSM tools on an MCP server
func Register(s *mcpserver.Server) {
	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "fsm.create_work_order",
		Description: "Dispatch field technician and create work order in Field Service Management system",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","asset_id","priority","idempotency_key"]}`),
		Handler: func(args map[string]any) (any, error) {
			assetID, _ := args["asset_id"].(string)
			priority, _ := args["priority"].(string)
			idempotencyKey, _ := args["idempotency_key"].(string)

			workOrdersMu.Lock()
			defer workOrdersMu.Unlock()

			if existing, ok := workOrders[idempotencyKey]; ok {
				return existing, nil
			}

			workOrderID := fmt.Sprintf("WO-%s-10423", assetID)
			res := map[string]any{
				"work_order_id":   workOrderID,
				"asset_id":        assetID,
				"priority":        priority,
				"status":          "ASSIGNED",
				"assigned_tech":   "Field Specialist - Manchester Team",
				"idempotency_key": idempotencyKey,
			}
			workOrders[idempotencyKey] = res
			return res, nil
		},
	})
}
