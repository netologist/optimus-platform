package infra_test

import (
	"context"
	"testing"
	"time"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/domain"
	"github.com/optimus/projects/platform/internal/tenant"
)

func TestStorageRLSAndOutboxIntegration(t *testing.T) {
	storage := app.NewMemoryStorage()

	// 1. Tenant Acme writes data
	ctxAcme := tenant.WithTenant(context.Background(), "acme")
	wo := &domain.WorkOrder{
		ID:             "wo-101",
		TenantID:       "acme",
		AssetID:        "P-104",
		Priority:       "P1",
		Status:         "OPEN",
		IdempotencyKey: "test-key-101",
		CreatedAt:      time.Now().UTC(),
	}

	if err := storage.SaveWorkOrder(ctxAcme, wo); err != nil {
		t.Fatalf("failed to save work order: %v", err)
	}

	// 2. Tenant Globex attempts to overwrite or save with mismatched tenant
	ctxGlobex := tenant.WithTenant(context.Background(), "globex")
	if err := storage.SaveWorkOrder(ctxGlobex, wo); err == nil {
		t.Fatalf("expected RLS rejection when saving tenant acme work order from tenant globex context")
	}

	// 3. Outbox persistence check
	msg := &domain.OutboxMessage{
		TenantID:      "acme",
		EventType:     "work_order.created",
		CorrelationID: "test-key-101",
		Traceparent:   "00-traceparent-test",
		Payload:       []byte(`{"work_order_id":"wo-101"}`),
		CreatedAt:     time.Now().UTC(),
	}
	if err := storage.SaveOutboxMessage(ctxAcme, msg); err != nil {
		t.Fatalf("failed to save outbox message: %v", err)
	}

	unpub, err := storage.GetUnpublishedOutboxMessages(context.Background(), 10)
	if err != nil || len(unpub) != 1 {
		t.Fatalf("expected 1 unpublished message, got %d (err: %v)", len(unpub), err)
	}
}
