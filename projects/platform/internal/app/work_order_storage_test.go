package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/domain"
	"github.com/optimus/projects/platform/internal/tenant"
)

// SaveWorkOrderWithEvent is the seam that keeps the work order row and its domain event
// from diverging. These tests pin both halves of that contract.
func TestSaveWorkOrderWithEventWritesBothRowAndEvent(t *testing.T) {
	storage := app.NewMemoryStorage()
	ctx := tenant.WithTenant(context.Background(), "acme")

	wo := &domain.WorkOrder{
		ID:             "WO-10423",
		TenantID:       "acme",
		AssetID:        "P-104",
		Priority:       "P1",
		Status:         "OPEN",
		IdempotencyKey: "wf-run-123",
		CreatedAt:      time.Now().UTC(),
	}
	msg := &domain.OutboxMessage{
		TenantID:      "acme",
		EventType:     "work_order.created",
		CorrelationID: "wf-run-123",
		Traceparent:   "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		Payload:       json.RawMessage(`{"work_order_id":"WO-10423"}`),
		CreatedAt:     time.Now().UTC(),
	}

	if err := storage.SaveWorkOrderWithEvent(ctx, wo, msg); err != nil {
		t.Fatalf("SaveWorkOrderWithEvent returned error: %v", err)
	}

	got, err := storage.GetWorkOrder(ctx, "acme", "WO-10423")
	if err != nil {
		t.Fatalf("work order was not persisted: %v", err)
	}
	if got.Status != "OPEN" || got.IdempotencyKey != "wf-run-123" {
		t.Errorf("persisted work order mismatch: %+v", got)
	}

	pending, err := storage.GetUnpublishedOutboxMessages(ctx, 10)
	if err != nil {
		t.Fatalf("GetUnpublishedOutboxMessages returned error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected exactly 1 pending event, got %d", len(pending))
	}
	if pending[0].EventType != "work_order.created" {
		t.Errorf("expected event type work_order.created, got %s", pending[0].EventType)
	}
	if pending[0].CorrelationID != "wf-run-123" {
		t.Errorf("expected correlation id wf-run-123, got %s", pending[0].CorrelationID)
	}
	// The trace must survive into the outbox so the relay can republish it (ADR-019).
	if pending[0].Traceparent == "" {
		t.Error("expected traceparent to be carried into the outbox record")
	}
}

func TestSaveWorkOrderWithEventRejectsCrossTenantWrite(t *testing.T) {
	storage := app.NewMemoryStorage()
	ctx := tenant.WithTenant(context.Background(), "acme")

	wo := &domain.WorkOrder{ID: "WO-1", TenantID: "globex", AssetID: "P-1", Status: "OPEN"}
	msg := &domain.OutboxMessage{TenantID: "globex", EventType: "work_order.created"}

	if err := storage.SaveWorkOrderWithEvent(ctx, wo, msg); err == nil {
		t.Fatal("expected RLS violation when writing another tenant's work order, got nil")
	}

	// The rejected write must not leave a partially applied row behind.
	if _, err := storage.GetWorkOrder(tenant.WithTenant(context.Background(), "globex"), "globex", "WO-1"); err == nil {
		t.Fatal("expected no work order to be persisted for the rejected tenant")
	}
}
