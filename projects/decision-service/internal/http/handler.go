package http

import (
	"encoding/json"
	"net/http"

	"github.com/optimus/projects/decision-service/internal/decision"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	svc *decision.Service
	mux *http.ServeMux
}

func NewHandler(svc *decision.Service) *Handler {
	h := &Handler{
		svc: svc,
		mux: http.NewServeMux(),
	}
	h.routes()
	return h
}

func (h *Handler) routes() {
	h.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	h.mux.HandleFunc("/v1/decisions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		tr := otel.Tracer("decision-service")
		ctx, span := tr.Start(ctx, "RunDecision",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
			),
		)
		defer span.End()

		var req decision.DecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			span.RecordError(err)
			http.Error(w, `{"error":"invalid request payload"}`, http.StatusBadRequest)
			return
		}

		if req.TenantID == "" {
			req.TenantID = r.Header.Get("X-Tenant-ID")
		}
		span.SetAttributes(
			attribute.String("tenant_id", req.TenantID),
			attribute.String("asset_id", req.AssetID),
		)

		govDecision, err := h.svc.Decide(ctx, req)
		if err != nil {
			span.RecordError(err)
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		span.SetAttributes(
			attribute.String("severity", govDecision.Severity),
			attribute.Bool("requires_approval", govDecision.RequiresApproval),
			attribute.Float64("confidence", govDecision.Confidence),
		)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(govDecision)
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
