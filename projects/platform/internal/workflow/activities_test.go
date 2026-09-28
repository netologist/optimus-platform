package workflow_test

import (
	"context"
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/tenant"
	"github.com/optimus/projects/platform/internal/workflow"
)

// The work order must be recorded in the platform's own tables, not just dispatched to
// the FSM mock, otherwise no work_order.created event ever reaches Redpanda.
func TestCreateFieldWorkOrderPersistsRowAndEvent(t *testing.T) {
	storage := app.NewMemoryStorage()
	// Port 1 refuses connections, so the activity exercises its dispatch-failure path.
	acts := workflow.NewActivities("", "", "http://127.0.0.1:1", storage)

	out, err := acts.CreateFieldWorkOrder(context.Background(), "acme", "P-104", "P1", "wf-run-123")
	if err != nil {
		t.Fatalf("CreateFieldWorkOrder returned error: %v", err)
	}
	if out.WorkOrderID == "" {
		t.Fatal("expected a work order id")
	}

	ctx := tenant.WithTenant(context.Background(), "acme")

	wo, err := storage.GetWorkOrder(ctx, "acme", out.WorkOrderID)
	if err != nil {
		t.Fatalf("work order was not persisted: %v", err)
	}
	if wo.AssetID != "P-104" || wo.Priority != "P1" || wo.Status != "OPEN" {
		t.Errorf("persisted work order mismatch: %+v", wo)
	}
	if wo.IdempotencyKey != "wf-run-123" {
		t.Errorf("expected idempotency key wf-run-123, got %s", wo.IdempotencyKey)
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
		t.Errorf("expected correlation id to be the workflow run id, got %s", pending[0].CorrelationID)
	}
	if pending[0].TenantID != "acme" {
		t.Errorf("expected tenant acme on the event, got %s", pending[0].TenantID)
	}
}

// Without storage the activity must still dispatch, so the worker can run in modes that
// do not own the database.
func TestCreateFieldWorkOrderWithoutStorageStillDispatches(t *testing.T) {
	acts := workflow.NewActivities("", "", "http://127.0.0.1:1", nil)

	out, err := acts.CreateFieldWorkOrder(context.Background(), "acme", "P-104", "P1", "wf-run-456")
	if err != nil {
		t.Fatalf("CreateFieldWorkOrder returned error: %v", err)
	}
	if out.WorkOrderID == "" {
		t.Fatal("expected a work order id")
	}
}
