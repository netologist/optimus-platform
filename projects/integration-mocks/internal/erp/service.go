package erp

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
)

var (
	reservationsMu sync.Mutex
	reservations   = make(map[string]map[string]any)
)

// Register registers ERP tools on an MCP server
func Register(s *mcpserver.Server) {
	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "erp.get_inventory",
		Description: "Check current spare part warehouse stock level and lead time",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","part_id"]}`),
		Handler: func(args map[string]any) (any, error) {
			partID, _ := args["part_id"].(string)
			if partID == "" {
				return nil, fmt.Errorf("part_id is required")
			}
			return map[string]any{
				"part_id":   partID,
				"name":      "Thermostat Replacement Unit",
				"in_stock":  6,
				"warehouse": "Manchester Warehouse A",
				"lead_days": 0,
			}, nil
		},
	})

	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "erp.reserve_inventory",
		Description: "Reserve spare parts in ERP warehouse with idempotency key",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","part_id","quantity","idempotency_key"]}`),
		Handler: func(args map[string]any) (any, error) {
			partID, _ := args["part_id"].(string)
			idempotencyKey, _ := args["idempotency_key"].(string)

			reservationsMu.Lock()
			defer reservationsMu.Unlock()

			if existing, ok := reservations[idempotencyKey]; ok {
				return existing, nil
			}

			reservationID := fmt.Sprintf("RES-%s-%s", partID, idempotencyKey[:min(8, len(idempotencyKey))])
			res := map[string]any{
				"reservation_id":  reservationID,
				"part_id":         partID,
				"quantity":        1,
				"status":          "RESERVED",
				"idempotency_key": idempotencyKey,
			}
			reservations[idempotencyKey] = res
			return res, nil
		},
	})

	s.RegisterTool(mcpserver.ToolDefinition{
		Name:        "erp.release_inventory_reservation",
		Description: "Compensating Saga action: Release previously reserved spare parts back to available stock",
		InputSchema: json.RawMessage(`{"type":"object","required":["tenant_id","reservation_id"]}`),
		Handler: func(args map[string]any) (any, error) {
			reservationID, _ := args["reservation_id"].(string)
			if reservationID == "" {
				return nil, fmt.Errorf("reservation_id is required")
			}

			reservationsMu.Lock()
			defer reservationsMu.Unlock()

			// Remove or mark released
			for k, v := range reservations {
				if v["reservation_id"] == reservationID {
					delete(reservations, k)
					break
				}
			}

			return map[string]any{
				"reservation_id": reservationID,
				"status":         "RELEASED",
				"compensated":    true,
			}, nil
		},
	})
}
