package decision_test

import (
	"context"
	"testing"

	"github.com/optimus/projects/decision-service/internal/audit"
	"github.com/optimus/projects/decision-service/internal/decision"
	"github.com/optimus/projects/decision-service/internal/policy"
)

func TestDecisionEvaluationAndPolicyAudit(t *testing.T) {
	engine := policy.NewEngine()
	auditStore := audit.NewMemoryStore()

	// Using nil client to exercise deterministic simulation
	svc := decision.NewService(nil, engine, auditStore, "laya")

	req := decision.DecisionRequest{
		TenantID: "acme",
		AssetID:  "P-104",
		State: map[string]any{
			"failures_last_30_days": 4,
			"plm_findings":          "thermostat failure",
		},
	}

	govDecision, err := svc.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error from Decide: %v", err)
	}

	// 1. Assert structured outcome
	if govDecision.Severity != "P1" {
		t.Errorf("expected severity P1, got %s", govDecision.Severity)
	}
	if govDecision.SafetyRisk != "HIGH" {
		t.Errorf("expected safety_risk HIGH, got %s", govDecision.SafetyRisk)
	}
	if !govDecision.FieldVisitRequired {
		t.Errorf("expected field_visit_required true")
	}
	if !govDecision.RequiresApproval {
		t.Errorf("expected requires_approval true due to HIGH safety risk")
	}
	if govDecision.PolicyVersion != "policy_v1" {
		t.Errorf("expected policy_version policy_v1, got %s", govDecision.PolicyVersion)
	}

	// 2. Assert Audit persistence
	record, err := auditStore.Get(context.Background(), "acme", govDecision.DecisionID)
	if err != nil || record == nil {
		t.Fatalf("expected audit record to be saved for decision %s", govDecision.DecisionID)
	}
	if record.AssetID != "P-104" {
		t.Errorf("expected asset P-104 in audit, got %s", record.AssetID)
	}
	if record.RequiresApproval != true {
		t.Errorf("expected audit record requires_approval true")
	}
}
