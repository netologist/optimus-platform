package transport

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/tenant"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Handler serves Platform HTTP API
type Handler struct {
	svc *app.Service
	mux *http.ServeMux
}

func NewHandler(svc *app.Service) *Handler {
	h := &Handler{
		svc: svc,
		mux: http.NewServeMux(),
	}
	h.routes()
	return h
}

func (h *Handler) routes() {
	// Root health
	h.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// OpenAPI 3.0 specification & Swagger UI documentation
	h.mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(GetOpenAPISpec())
	})

	h.mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(GetSwaggerUIHTML())
	})

	// Wrap tenant-scoped routes with Tenant Middleware
	tenantMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/signals") && r.Method == http.MethodPost:
			h.handleSignal(w, r)
		case strings.HasSuffix(path, "/tools") && r.Method == http.MethodGet:
			h.handleTools(w, r)
		case strings.Contains(path, "/approvals/") && strings.HasSuffix(path, "/approve") && r.Method == http.MethodPost:
			h.handleApprovalDecision(w, r, true)
		case strings.Contains(path, "/approvals/") && strings.HasSuffix(path, "/reject") && r.Method == http.MethodPost:
			h.handleApprovalDecision(w, r, false)
		default:
			http.NotFound(w, r)
		}
	})

	h.mux.Handle("/v1/tenants/", tenant.Middleware(tenantMux))
}

func (h *Handler) handleSignal(w http.ResponseWriter, r *http.Request) {
	// Extract incoming W3C traceparent carrier
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	tr := otel.Tracer("platform-api")
	ctx, span := tr.Start(ctx, "IngestSignal",
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.path", r.URL.Path),
		),
	)
	defer span.End()

	var req app.IngestSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("asset_id", req.AssetID),
		attribute.String("symptom", req.Symptom),
	)

	traceparent := r.Header.Get("traceparent")
	resp, err := h.svc.IngestSignal(ctx, req, traceparent)
	if err != nil {
		span.RecordError(err)
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	span.SetAttributes(
		attribute.String("signal_id", resp.SignalID),
		attribute.String("workflow_id", resp.WorkflowID),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleTools(w http.ResponseWriter, r *http.Request) {
	tools, err := h.svc.GetActiveTools(r.Context())
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tools": tools})
}

func (h *Handler) handleApprovalDecision(w http.ResponseWriter, r *http.Request, approved bool) {
	// Extract workflow ID from path: /v1/tenants/{tenant_id}/approvals/{workflow_id}/(approve|reject)
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var workflowID string
	for i, p := range parts {
		if p == "approvals" && i+1 < len(parts) {
			workflowID = parts[i+1]
			break
		}
	}

	if workflowID == "" {
		http.Error(w, `{"error":"missing workflow_id in path"}`, http.StatusBadRequest)
		return
	}

	var reqBody struct {
		Approver string `json:"approver"`
		Notes    string `json:"notes"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
	}

	resp, err := h.svc.ApproveWorkflow(r.Context(), app.ApproveWorkflowRequest{
		WorkflowID: workflowID,
		Approved:   approved,
		Approver:   reqBody.Approver,
	})
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
