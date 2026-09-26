package temporal

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"

	"github.com/optimus/projects/platform/internal/workflow"
)

// Client wraps the Temporal Go SDK client to start and signal workflows
type Client struct {
	tc        client.Client
	taskQueue string
}

func NewClient(tc client.Client) *Client {
	return &Client{
		tc:        tc,
		taskQueue: "optimus-task-queue",
	}
}

func (c *Client) StartAssetFailureWorkflow(ctx context.Context, workflowID, tenantID, assetID, symptom, traceparent string) error {
	if c.tc == nil {
		return nil
	}

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: c.taskQueue,
	}

	input := workflow.AssetFailureWorkflowInput{
		AssetID:     assetID,
		TenantID:    tenantID,
		Symptom:     symptom,
		Traceparent: traceparent,
	}

	_, err := c.tc.ExecuteWorkflow(ctx, options, workflow.AssetFailureWorkflow, input)
	if err != nil {
		return fmt.Errorf("failed to start AssetFailureWorkflow (%s): %w", workflowID, err)
	}
	return nil
}

func (c *Client) SignalApproval(ctx context.Context, workflowID string, approved bool, approver string) error {
	if c.tc == nil {
		return nil
	}

	signal := workflow.ApprovalSignal{
		Approved: approved,
		Approver: approver,
	}

	err := c.tc.SignalWorkflow(ctx, workflowID, "", workflow.ApprovalSignalName, signal)
	if err != nil {
		return fmt.Errorf("failed to signal workflow approval (%s): %w", workflowID, err)
	}
	return nil
}

// NoopClient is used when running without a live Temporal cluster
type NoopClient struct{}

func NewNoopClient() *NoopClient {
	return &NoopClient{}
}

func (n *NoopClient) StartAssetFailureWorkflow(ctx context.Context, workflowID, tenantID, assetID, symptom, traceparent string) error {
	return nil
}

func (n *NoopClient) SignalApproval(ctx context.Context, workflowID string, approved bool, approver string) error {
	return nil
}
