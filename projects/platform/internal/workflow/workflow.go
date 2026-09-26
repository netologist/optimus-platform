package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ApprovalSignalName = "approval-granted"
	WorkflowName       = "AssetFailureWorkflow"
)

// AssetFailureWorkflow orchestrates the end-to-end asset investigation, decisioning, and operational dispatch
func AssetFailureWorkflow(ctx workflow.Context, input AssetFailureWorkflowInput) (*AssetFailureWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetFailureWorkflow", "asset_id", input.AssetID, "tenant_id", input.TenantID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 90 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var acts *Activities

	// Step 1: Investigate Failure (AI Runtime)
	var evidence EvidenceContext
	err := workflow.ExecuteActivity(ctx, acts.InvestigateFailure, input).Get(ctx, &evidence)
	if err != nil {
		return nil, fmt.Errorf("InvestigateFailure activity failed: %w", err)
	}

	// Step 2: Run Decision (Ollaya + Policy Engine)
	var decision GovernedDecision
	err = workflow.ExecuteActivity(ctx, acts.RunDecision, &evidence).Get(ctx, &decision)
	if err != nil {
		return nil, fmt.Errorf("RunDecision activity failed: %w", err)
	}

	// Step 3: Human-in-the-loop Approval if required
	if decision.RequiresApproval {
		logger.Info("Workflow requires human approval", "reason", decision.ApprovalReason)

		signalChan := workflow.GetSignalChannel(ctx, ApprovalSignalName)
		var approval ApprovalSignal

		// 24 hour timeout
		timer := workflow.NewTimer(ctx, 24*time.Hour)

		selector := workflow.NewSelector(ctx)
		selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, &approval)
		})
		selector.AddFuture(timer, func(f workflow.Future) {
			logger.Warn("Approval timed out after 24 hours")
			approval = ApprovalSignal{Approved: false}
		})

		selector.Select(ctx)

		if !approval.Approved {
			return &AssetFailureWorkflowOutput{
				WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
				Status:     "REJECTED",
			}, nil
		}
		logger.Info("Approval granted", "approver", approval.Approver)
	}

	wfRunID := workflow.GetInfo(ctx).WorkflowExecution.RunID

	// Step 4: Reserve Spare Part (ERP)
	var reserveOut ReservePartOutput
	err = workflow.ExecuteActivity(ctx, acts.ReserveSparePart, input.TenantID, evidence.SparePartID, wfRunID).Get(ctx, &reserveOut)
	if err != nil {
		return nil, fmt.Errorf("ReserveSparePart activity failed: %w", err)
	}

	// Step 5: Create Field Work Order (FSM) with Saga Compensation
	var woOut CreateWorkOrderOutput
	err = workflow.ExecuteActivity(ctx, acts.CreateFieldWorkOrder, input.TenantID, input.AssetID, decision.Severity, wfRunID).Get(ctx, &woOut)
	if err != nil {
		logger.Error("CreateFieldWorkOrder failed, triggering Saga compensation", "error", err)

		// Saga Compensating Activity: Release spare part reservation
		compCtx, _ := workflow.NewDisconnectedContext(ctx)
		_ = workflow.ExecuteActivity(compCtx, acts.ReleaseSparePartReservation, input.TenantID, reserveOut.ReservationID).Get(compCtx, nil)

		return &AssetFailureWorkflowOutput{
			WorkflowID:    workflow.GetInfo(ctx).WorkflowExecution.ID,
			Status:        "FAILED_COMPENSATED",
			ReservationID: reserveOut.ReservationID,
			Compensated:   true,
		}, nil
	}

	logger.Info("AssetFailureWorkflow successfully completed", "work_order_id", woOut.WorkOrderID)
	return &AssetFailureWorkflowOutput{
		WorkflowID:    workflow.GetInfo(ctx).WorkflowExecution.ID,
		Status:        "COMPLETED",
		WorkOrderID:   woOut.WorkOrderID,
		ReservationID: reserveOut.ReservationID,
	}, nil
}
