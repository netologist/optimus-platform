package mcpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/optimus/projects/integration-mocks/internal/eam"
	"github.com/optimus/projects/integration-mocks/internal/erp"
	"github.com/optimus/projects/integration-mocks/internal/fsm"
	"github.com/optimus/projects/integration-mocks/internal/mcpserver"
	"github.com/optimus/projects/integration-mocks/internal/plm"
)

func TestMCPServerToolsListAndCall(t *testing.T) {
	srv := mcpserver.New("test-server")
	eam.Register(srv)
	plm.Register(srv)
	erp.Register(srv)
	fsm.Register(srv)

	// 1. Test tools/list
	listReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	body, _ := json.Marshal(listReq)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var listResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Result.Tools) < 5 {
		t.Errorf("expected at least 5 registered tools, got %d", len(listResp.Result.Tools))
	}

	// 2. Test tools/call: eam.get_maintenance_history
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "eam.get_maintenance_history",
			"arguments": map[string]any{
				"tenant_id": "acme",
				"asset_id":  "P-104",
			},
		},
	}
	body, _ = json.Marshal(callReq)
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec = httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var callResp struct {
		Result struct {
			Data map[string]any `json:"data"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &callResp)
	failures := callResp.Result.Data["failures_last_30_days"]
	if failures != float64(4) {
		t.Errorf("expected 4 failures, got %v", failures)
	}

	// 3. Test call log endpoint
	req = httptest.NewRequest(http.MethodGet, "/call-log", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var callLogs []mcpserver.CallRecord
	_ = json.Unmarshal(rec.Body.Bytes(), &callLogs)
	if len(callLogs) != 1 || callLogs[0].Tool != "eam.get_maintenance_history" {
		t.Errorf("unexpected call logs: %+v", callLogs)
	}
}

func TestSagaReservationAndCompensation(t *testing.T) {
	srv := mcpserver.New("erp")
	erp.Register(srv)

	// 1. Reserve Inventory
	reserveReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "erp.reserve_inventory",
			"arguments": map[string]any{
				"tenant_id":       "acme",
				"part_id":         "SP-COOL-9981",
				"quantity":        1,
				"idempotency_key": "wf-test-1",
			},
		},
	}
	body, _ := json.Marshal(reserveReq)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))

	var reserveResp struct {
		Result struct {
			Data map[string]any `json:"data"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reserveResp)
	reservationID := reserveResp.Result.Data["reservation_id"].(string)
	if reservationID == "" {
		t.Fatalf("expected reservation_id, got empty")
	}

	// 2. Compensate / Release Reservation
	releaseReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "erp.release_inventory_reservation",
			"arguments": map[string]any{
				"tenant_id":      "acme",
				"reservation_id": reservationID,
			},
		},
	}
	body, _ = json.Marshal(releaseReq)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))

	var releaseResp struct {
		Result struct {
			Data map[string]any `json:"data"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &releaseResp)
	if releaseResp.Result.Data["compensated"] != true {
		t.Errorf("expected compensated=true, got %v", releaseResp.Result.Data["compensated"])
	}
}
