package temporal_test

import (
	"context"
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/temporal"
)

func TestNoopClientImplementsWorkflowClient(t *testing.T) {
	var client app.WorkflowClient = temporal.NewNoopClient()
	ctx := context.Background()

	err := client.StartAssetFailureWorkflow(ctx, "wf-1", "acme", "P-104", "overheating", "00-trace")
	if err != nil {
		t.Fatalf("expected nil error from noop client, got %v", err)
	}

	err = client.SignalApproval(ctx, "wf-1", true, "j.smith")
	if err != nil {
		t.Fatalf("expected nil error from noop client, got %v", err)
	}
}

func TestClientWithNilTemporalClientDoesNotPanic(t *testing.T) {
	client := temporal.NewClient(nil)
	var _ app.WorkflowClient = client
	ctx := context.Background()

	err := client.StartAssetFailureWorkflow(ctx, "wf-1", "acme", "P-104", "overheating", "00-trace")
	if err != nil {
		t.Fatalf("expected nil error from client with nil tc, got %v", err)
	}

	err = client.SignalApproval(ctx, "wf-1", true, "j.smith")
	if err != nil {
		t.Fatalf("expected nil error from client with nil tc, got %v", err)
	}
}
