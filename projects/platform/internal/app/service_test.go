package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/tenant"
)

type testWorkflowClient struct {
	startedWorkflowID string
	startedTenantID   string
	startedAssetID    string
	startedSymptom    string
	startedTrace      string
	signaledID        string
	signaledApproved  bool
	signaledApprover  string
}

func (m *testWorkflowClient) StartAssetFailureWorkflow(ctx context.Context, workflowID, tenantID, assetID, symptom, traceparent string) error {
	m.startedWorkflowID = workflowID
	m.startedTenantID = tenantID
	m.startedAssetID = assetID
	m.startedSymptom = symptom
	m.startedTrace = traceparent
	return nil
}

func (m *testWorkflowClient) SignalApproval(ctx context.Context, workflowID string, approved bool, approver string) error {
	m.signaledID = workflowID
	m.signaledApproved = approved
	m.signaledApprover = approver
	return nil
}

func TestIngestSignalAndRLS(t *testing.T) {
	storage := app.NewMemoryStorage()
	wfClient := &testWorkflowClient{}
	svc := app.NewService(storage, app.WithWorkflowClient(wfClient))

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

	// Verify WorkflowClient received the start invocation
	if wfClient.startedWorkflowID != resp.WorkflowID {
		t.Errorf("expected workflow started with ID %s, got %s", resp.WorkflowID, wfClient.startedWorkflowID)
	}
	if wfClient.startedAssetID != "P-104" || wfClient.startedTenantID != "acme" {
		t.Errorf("expected workflow started for acme/P-104, got %s/%s", wfClient.startedTenantID, wfClient.startedAssetID)
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

func TestApproveWorkflow(t *testing.T) {
	storage := app.NewMemoryStorage()
	wfClient := &testWorkflowClient{}
	svc := app.NewService(storage, app.WithWorkflowClient(wfClient))
	ctxAcme := tenant.WithTenant(context.Background(), "acme")

	// 1. Approve
	appResp, err := svc.ApproveWorkflow(ctxAcme, app.ApproveWorkflowRequest{
		WorkflowID: "wf-1234",
		Approved:   true,
		Approver:   "lead.engineer@acme.com",
	})
	if err != nil {
		t.Fatalf("unexpected error approving workflow: %v", err)
	}
	if appResp.Status != "approved" || !appResp.Signaled {
		t.Errorf("expected approved with signaled=true, got %+v", appResp)
	}
	if wfClient.signaledID != "wf-1234" || !wfClient.signaledApproved || wfClient.signaledApprover != "lead.engineer@acme.com" {
		t.Errorf("unexpected signal values in mock: %+v", wfClient)
	}

	// 2. Reject
	rejResp, err := svc.ApproveWorkflow(ctxAcme, app.ApproveWorkflowRequest{
		WorkflowID: "wf-1234",
		Approved:   false,
	})
	if err != nil {
		t.Fatalf("unexpected error rejecting workflow: %v", err)
	}
	if rejResp.Status != "rejected" || !rejResp.Signaled {
		t.Errorf("expected rejected with signaled=true, got %+v", rejResp)
	}
	if wfClient.signaledApproved {
		t.Errorf("expected false for rejection signal")
	}

	// 3. Missing tenant context
	_, err = svc.ApproveWorkflow(context.Background(), app.ApproveWorkflowRequest{
		WorkflowID: "wf-1234",
		Approved:   true,
	})
	if err == nil {
		t.Fatalf("expected error without tenant context, got nil")
	}
}

func TestGetActiveTools(t *testing.T) {
	storage := app.NewMemoryStorage()
	svc := app.NewService(storage)
	ctx := tenant.WithTenant(context.Background(), "acme")

	tools, err := svc.GetActiveTools(ctx)
	if err != nil {
		t.Fatalf("unexpected error getting tools: %v", err)
	}
	if len(tools) == 0 {
		t.Fatalf("expected tools list to not be empty")
	}

	hasEAM := false
	hasPLM := false
	for _, tool := range tools {
		if strings.HasPrefix(tool.Name, "eam.") {
			hasEAM = true
		}
		if strings.HasPrefix(tool.Name, "plm.") {
			hasPLM = true
		}
	}
	if !hasEAM || !hasPLM {
		t.Errorf("expected both EAM and PLM tools in discovery, got %v", tools)
	}
}
