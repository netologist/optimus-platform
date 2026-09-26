package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/optimus/projects/platform/internal/workflow"
)

type WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *WorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *WorkflowTestSuite) TearDownTest() {
	s.env.AssertExpectations(s.T())
}

func (s *WorkflowTestSuite) TestAssetFailureWorkflow_SuccessWithApproval() {
	var acts *workflow.Activities

	s.env.RegisterActivity(acts.InvestigateFailure)
	s.env.RegisterActivity(acts.RunDecision)
	s.env.RegisterActivity(acts.ReserveSparePart)
	s.env.RegisterActivity(acts.CreateFieldWorkOrder)
	s.env.RegisterActivity(acts.ReleaseSparePartReservation)

	s.env.OnActivity(acts.InvestigateFailure, mock.Anything, mock.Anything).Return(&workflow.EvidenceContext{
		AssetID:             "P-104",
		TenantID:            "acme",
		FailuresLast30Days:  4,
		SparePartID:         "SP-COOL-9981",
		SparePartInStock:    true,
	}, nil)

	s.env.OnActivity(acts.RunDecision, mock.Anything, mock.Anything).Return(&workflow.GovernedDecision{
		DecisionID:       "dec-104",
		Severity:         "P1",
		SafetyRisk:       "HIGH",
		RequiresApproval: true,
	}, nil)

	s.env.OnActivity(acts.ReserveSparePart, mock.Anything, "acme", "SP-COOL-9981", mock.Anything).Return(&workflow.ReservePartOutput{
		ReservationID: "RES-9981",
		PartID:        "SP-COOL-9981",
	}, nil)

	s.env.OnActivity(acts.CreateFieldWorkOrder, mock.Anything, "acme", "P-104", "P1", mock.Anything).Return(&workflow.CreateWorkOrderOutput{
		WorkOrderID: "WO-10423",
		AssetID:     "P-104",
		Priority:    "P1",
	}, nil)

	// Send approval signal
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(workflow.ApprovalSignalName, workflow.ApprovalSignal{
			Approved: true,
			Approver: "j.smith",
		})
	}, 1)

	s.env.ExecuteWorkflow(workflow.AssetFailureWorkflow, workflow.AssetFailureWorkflowInput{
		TenantID: "acme",
		AssetID:  "P-104",
		Symptom:  "overheating",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var out workflow.AssetFailureWorkflowOutput
	s.NoError(s.env.GetWorkflowResult(&out))
	s.Equal("COMPLETED", out.Status)
	s.Equal("WO-10423", out.WorkOrderID)
	s.Equal("RES-9981", out.ReservationID)
}

func (s *WorkflowTestSuite) TestAssetFailureWorkflow_SagaCompensationOnFSMError() {
	var acts *workflow.Activities

	s.env.RegisterActivity(acts.InvestigateFailure)
	s.env.RegisterActivity(acts.RunDecision)
	s.env.RegisterActivity(acts.ReserveSparePart)
	s.env.RegisterActivity(acts.CreateFieldWorkOrder)
	s.env.RegisterActivity(acts.ReleaseSparePartReservation)

	s.env.OnActivity(acts.InvestigateFailure, mock.Anything, mock.Anything).Return(&workflow.EvidenceContext{
		AssetID:          "P-104",
		TenantID:         "acme",
		SparePartID:      "SP-COOL-9981",
		SparePartInStock: true,
	}, nil)

	s.env.OnActivity(acts.RunDecision, mock.Anything, mock.Anything).Return(&workflow.GovernedDecision{
		DecisionID:       "dec-104",
		RequiresApproval: false,
	}, nil)

	s.env.OnActivity(acts.ReserveSparePart, mock.Anything, "acme", "SP-COOL-9981", mock.Anything).Return(&workflow.ReservePartOutput{
		ReservationID: "RES-9981",
		PartID:        "SP-COOL-9981",
	}, nil)

	// Simulate FSM permanent failure
	s.env.OnActivity(acts.CreateFieldWorkOrder, mock.Anything, "acme", "P-104", mock.Anything, mock.Anything).Return(nil, errors.New("FSM service unavailable"))

	// Expect Saga compensating activity
	s.env.OnActivity(acts.ReleaseSparePartReservation, mock.Anything, "acme", "RES-9981").Return(nil)

	s.env.ExecuteWorkflow(workflow.AssetFailureWorkflow, workflow.AssetFailureWorkflowInput{
		TenantID: "acme",
		AssetID:  "P-104",
		Symptom:  "overheating",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var out workflow.AssetFailureWorkflowOutput
	s.NoError(s.env.GetWorkflowResult(&out))
	s.Equal("FAILED_COMPENSATED", out.Status)
	s.True(out.Compensated)
	s.Equal("RES-9981", out.ReservationID)
}

func TestWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowTestSuite))
}
