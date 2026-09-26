package app_test

import (
	"context"
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/tenant"
)

func TestIngestSignalAndRLS(t *testing.T) {
	storage := app.NewMemoryStorage()
	svc := app.NewService(storage)

	// 1. Ingest signal as tenant "acme"
	ctxAcme := tenant.WithTenant(context.Background(), "acme")
	resp, err := svc.IngestSignal(ctxAcme, app.IngestSignalRequest{
		AssetID: "P-104",
		Symptom: "repeated overheating",
	}, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	if err != nil {
		t.Fatalf("unexpected error ingesting signal: %v", err)
	}
	if resp.SignalID == "" || resp.WorkflowID == "" {
		t.Fatalf("expected signal_id and workflow_id, got %+v", resp)
	}

	// 2. Fetch signal as "acme" -> should succeed
	sig, err := storage.GetSignal(ctxAcme, "acme", resp.SignalID)
	if err != nil {
		t.Fatalf("expected to read signal, got error: %v", err)
	}
	if sig.AssetID != "P-104" {
		t.Errorf("expected asset P-104, got %s", sig.AssetID)
	}

	// 3. Attempt to fetch signal as "globex" -> should fail RLS check
	ctxGlobex := tenant.WithTenant(context.Background(), "globex")
	_, err = storage.GetSignal(ctxGlobex, "acme", resp.SignalID)
	if err == nil {
		t.Fatalf("expected RLS error accessing acme data from globex context, got nil")
	}

	// 4. Check outbox message exists
	msgs, err := storage.GetUnpublishedOutboxMessages(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to query outbox: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 unpublished outbox message, got %d", len(msgs))
	}
	if msgs[0].CorrelationID != resp.WorkflowID {
		t.Errorf("expected correlation ID %s, got %s", resp.WorkflowID, msgs[0].CorrelationID)
	}
	if msgs[0].Traceparent != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Errorf("expected traceparent preserved, got %s", msgs[0].Traceparent)
	}
}
