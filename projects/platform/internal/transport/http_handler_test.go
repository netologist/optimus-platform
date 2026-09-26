package transport_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/transport"
)

func TestPlatformHTTPRoutes(t *testing.T) {
	storage := app.NewMemoryStorage()
	svc := app.NewService(storage)
	handler := transport.NewHandler(svc)

	// 1. Healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// 2. Post signal with X-Tenant-ID
	signalPayload := map[string]string{
		"asset_id": "P-104",
		"symptom":  "overheating",
	}
	body, _ := json.Marshal(signalPayload)
	req = httptest.NewRequest(http.MethodPost, "/v1/tenants/acme/signals", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "acme")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Get tools
	req = httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/tools", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// 4. Approve workflow
	req = httptest.NewRequest(http.MethodPost, "/v1/tenants/acme/approvals/wf-12345/approve", nil)
	req.Header.Set("X-Tenant-ID", "acme")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}
