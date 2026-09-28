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

const workflowCorrelation = "wf-asset-failure-P-104-1"

func seedTrail(t *testing.T, storage *app.MemoryStorage) {
	t.Helper()
	ctx := context.Background()

	// Both events of one workflow run share the workflow id; a third event belongs to a
	// different run and must not leak into the filtered view.
	seed := []*domain.OutboxMessage{
		{TenantID: "acme", EventType: "signal.received", CorrelationID: workflowCorrelation,
			Payload: json.RawMessage(`{"signal_id":"sig-1"}`), CreatedAt: time.Now().UTC()},
		{TenantID: "acme", EventType: "work_order.created", CorrelationID: workflowCorrelation,
			Traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			Payload:     json.RawMessage(`{"work_order_id":"WO-P-104-10423"}`), CreatedAt: time.Now().UTC()},
		{TenantID: "acme", EventType: "signal.received", CorrelationID: "wf-other",
			Payload: json.RawMessage(`{"signal_id":"sig-2"}`), CreatedAt: time.Now().UTC()},
	}

	for _, msg := range seed {
		if err := storage.SaveOutboxMessage(ctx, msg); err != nil {
			t.Fatalf("seeding outbox message failed: %v", err)
		}
	}
}

// The audit view must return both halves of one workflow under a single correlation id —
// that is the whole point of aligning the signal and work order on the workflow id.
func TestGetAuditTrailJoinsSignalAndWorkOrderByCorrelation(t *testing.T) {
	storage := app.NewMemoryStorage()
	seedTrail(t, storage)
	svc := app.NewService(storage)

	trail, err := svc.GetAuditTrail(tenant.WithTenant(context.Background(), "acme"), workflowCorrelation)
	if err != nil {
		t.Fatalf("GetAuditTrail returned error: %v", err)
	}

	if len(trail.Entries) != 2 {
		t.Fatalf("expected 2 entries for the workflow, got %d", len(trail.Entries))
	}
	if trail.Entries[0].EventType != "signal.received" {
		t.Errorf("expected signal first, got %s", trail.Entries[0].EventType)
	}
	if trail.Entries[1].EventType != "work_order.created" {
		t.Errorf("expected work order second, got %s", trail.Entries[1].EventType)
	}
	if trail.Entries[1].Traceparent == "" {
		t.Error("expected the work order entry to carry its traceparent")
	}
}

func TestGetAuditTrailWithoutFilterReturnsAllTenantEvents(t *testing.T) {
	storage := app.NewMemoryStorage()
	seedTrail(t, storage)
	svc := app.NewService(storage)

	trail, err := svc.GetAuditTrail(tenant.WithTenant(context.Background(), "acme"), "")
	if err != nil {
		t.Fatalf("GetAuditTrail returned error: %v", err)
	}
	if len(trail.Entries) != 3 {
		t.Fatalf("expected all 3 tenant events, got %d", len(trail.Entries))
	}
}

func TestGetAuditTrailIsTenantScoped(t *testing.T) {
	storage := app.NewMemoryStorage()
	seedTrail(t, storage)
	svc := app.NewService(storage)

	trail, err := svc.GetAuditTrail(tenant.WithTenant(context.Background(), "globex"), workflowCorrelation)
	if err != nil {
		t.Fatalf("GetAuditTrail returned error: %v", err)
	}
	if len(trail.Entries) != 0 {
		t.Fatalf("expected no entries for another tenant, got %d", len(trail.Entries))
	}
}
