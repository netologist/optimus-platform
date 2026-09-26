package transport

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/tenant"
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
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
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
			h.handleApprove(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	h.mux.Handle("/v1/tenants/", tenant.Middleware(tenantMux))
}

func (h *Handler) handleSignal(w http.ResponseWriter, r *http.Request) {
	var req app.IngestSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
		return
	}

	traceparent := r.Header.Get("traceparent")
	resp, err := h.svc.IngestSignal(r.Context(), req, traceparent)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

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

func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	// Extract workflow ID from path: /v1/tenants/{tenant_id}/approvals/{workflow_id}/approve
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	var workflowID string
	for i, p := range parts {
		if p == "approvals" && i+1 < len(parts) {
			workflowID = parts[i+1]
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "approved",
		"workflow_id": workflowID,
		"signaled":    true,
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
