package transport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/transport"
)

type mockWorkflowClient struct {
	startedWorkflowID string
	startedTenantID   string
	startedAssetID    string
	startedSymptom    string
	startedTrace      string
	signaledID        string
	signaledApproved  bool
	signaledApprover  string
}

func (m *mockWorkflowClient) StartAssetFailureWorkflow(ctx context.Context, workflowID, tenantID, assetID, symptom, traceparent string) error {
	m.startedWorkflowID = workflowID
	m.startedTenantID = tenantID
	m.startedAssetID = assetID
	m.startedSymptom = symptom
	m.startedTrace = traceparent
	return nil
}

func (m *mockWorkflowClient) SignalApproval(ctx context.Context, workflowID string, approved bool, approver string) error {
	m.signaledID = workflowID
	m.signaledApproved = approved
	m.signaledApprover = approver
	return nil
}

func TestPlatformHTTPRoutes(t *testing.T) {
	storage := app.NewMemoryStorage()
	wfMock := &mockWorkflowClient{}
	svc := app.NewService(storage, app.WithWorkflowClient(wfMock))
	handler := transport.NewHandler(svc)

	// 1. Healthz probe
	t.Run("Healthz", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	// 2. OpenAPI JSON Spec endpoint
	t.Run("OpenAPISpecJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var spec map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
			t.Fatalf("failed to parse openapi json: %v", err)
		}
		if spec["openapi"] != "3.0.3" {
			t.Errorf("expected openapi 3.0.3, got %v", spec["openapi"])
		}
	})

	// 2b. OpenAPI Spec from external file via OPENAPI_SPEC_PATH
	t.Run("OpenAPISpecExternalFile", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "custom-openapi-*.json")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer func() { _ = os.Remove(tempFile.Name()) }()

		customJSON := `{"openapi":"3.0.3","info":{"title":"Custom Override API","version":"2.0.0"}}`
		if _, err := tempFile.WriteString(customJSON); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}
		_ = tempFile.Close()

		t.Setenv("OPENAPI_SPEC_PATH", tempFile.Name())
		specBytes := transport.GetOpenAPISpec()
		if !strings.Contains(string(specBytes), "Custom Override API") {
			t.Errorf("expected custom spec from file, got %s", string(specBytes))
		}
	})
	// 3. Swagger UI Docs HTML endpoint
	t.Run("SwaggerUIDocsHTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
			t.Errorf("expected SwaggerUIBundle in HTML body")
		}
	})

	// 4. Ingest signal with X-Tenant-ID and traceparent
	t.Run("IngestSignal", func(t *testing.T) {
		payload := map[string]string{
			"asset_id": "P-104",
			"symptom":  "repeated overheating anomaly",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/v1/tenants/acme/signals", bytes.NewReader(body))
		req.Header.Set("X-Tenant-ID", "acme")
		req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
		}

		var res map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if res["status"] != "INGESTED" {
			t.Errorf("expected status INGESTED, got %v", res["status"])
		}
		if wfMock.startedAssetID != "P-104" || wfMock.startedTenantID != "acme" {
			t.Errorf("expected workflow started for acme/P-104, got %s/%s", wfMock.startedTenantID, wfMock.startedAssetID)
		}
		if wfMock.startedTrace != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
			t.Errorf("expected traceparent preserved in workflow, got %s", wfMock.startedTrace)
		}
	})

	// 5. Dynamic MCP Tools discovery
	t.Run("GetActiveTools", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/tools", nil)
		req.Header.Set("X-Tenant-ID", "acme")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var res map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		tools, ok := res["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Fatalf("expected non-empty tools array, got %v", res)
		}
	})

	// 6. Workflow supervisor approval
	t.Run("ApproveWorkflow", func(t *testing.T) {
		reqBody := bytes.NewReader([]byte(`{"approver":"j.smith@acme.com","notes":"verified"}`))
		req := httptest.NewRequest(http.MethodPost, "/v1/tenants/acme/approvals/wf-p104-99/approve", reqBody)
		req.Header.Set("X-Tenant-ID", "acme")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var res map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if res["status"] != "approved" || res["signaled"] != true {
			t.Errorf("expected approved response, got %v", res)
		}
		if wfMock.signaledID != "wf-p104-99" || !wfMock.signaledApproved || wfMock.signaledApprover != "j.smith@acme.com" {
			t.Errorf("unexpected signal state in mock: %+v", wfMock)
		}
	})

	// 7. Workflow supervisor rejection
	t.Run("RejectWorkflow", func(t *testing.T) {
		reqBody := bytes.NewReader([]byte(`{"approver":"supervisor@acme.com","notes":"denied"}`))
		req := httptest.NewRequest(http.MethodPost, "/v1/tenants/acme/approvals/wf-p104-99/reject", reqBody)
		req.Header.Set("X-Tenant-ID", "acme")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var res map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if res["status"] != "rejected" || res["signaled"] != true {
			t.Errorf("expected rejected response, got %v", res)
		}
		if wfMock.signaledApproved {
			t.Errorf("expected false for rejected workflow signal")
		}
	})

	// 8. Tenant header mismatch returns 401 Unauthorized
	t.Run("TenantHeaderMismatch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/tenants/acme/tools", nil)
		req.Header.Set("X-Tenant-ID", "globex")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized on tenant mismatch, got %d", rec.Code)
		}
	})
}
