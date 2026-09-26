package policy

import (
	"fmt"
	"strings"

	"github.com/optimus/projects/decision-service/internal/systemone"
)

const CurrentPolicyVersion = "policy_v1"

// GovernedDecision encapsulates the calibrated inferences and deterministic business rules
type GovernedDecision struct {
	DecisionID         string  `json:"decision_id"`
	Severity           string  `json:"severity"`
	SafetyRisk         string  `json:"safety_risk"`
	FieldVisitRequired bool    `json:"field_visit_required"`
	Confidence         float64 `json:"confidence"`
	RequiresApproval   bool    `json:"requires_approval"`
	ApprovalReason     string  `json:"approval_reason,omitempty"`
	PolicyVersion      string  `json:"policy_version"`
}

// Engine evaluates business policy rules on top of SystemOne answers
type Engine struct {
	minConfidenceThreshold float64
}

func NewEngine() *Engine {
	return &Engine{
		minConfidenceThreshold: 0.75,
	}
}

// Evaluate applies deterministic policies to SystemOne raw answers
func (e *Engine) Evaluate(decisionID string, resp *systemone.Response) GovernedDecision {
	gd := GovernedDecision{
		DecisionID:    decisionID,
		Confidence:    resp.Confidence,
		PolicyVersion: CurrentPolicyVersion,
	}

	// 1. Evaluate Severity
	if ans, ok := resp.Answers["severity"]; ok {
		// Score mapped to P1, P2, P3
		if ans.Score >= 2.0 || fmt.Sprintf("%v", ans.Value) == "High" {
			gd.Severity = "P1"
		} else if ans.Score >= 1.0 {
			gd.Severity = "P2"
		} else {
			gd.Severity = "P3"
		}
	} else {
		gd.Severity = "P2"
	}

	// 2. Evaluate Safety Risk
	if ans, ok := resp.Answers["safety_risk"]; ok {
		val := strings.ToUpper(fmt.Sprintf("%v", ans.Value))
		gd.SafetyRisk = val
	} else {
		gd.SafetyRisk = "MEDIUM"
	}

	// 3. Evaluate Field Visit Required
	if ans, ok := resp.Answers["field_visit_required"]; ok {
		if fmt.Sprintf("%v", ans.Value) == "true" || ans.Probability > 0.5 {
			gd.FieldVisitRequired = true
		}
	}

	// 4. Deterministic Rule Checks (Audit-Enforced)
	// Rule A: HIGH safety risk unconditionally mandates human approval
	if gd.SafetyRisk == "HIGH" {
		gd.RequiresApproval = true
		gd.ApprovalReason = "HIGH safety risk detected: human approval mandated by policy_v1"
		return gd
	}

	// Rule B: P1 Severity with Field Visit requires approval
	if gd.Severity == "P1" && gd.FieldVisitRequired {
		gd.RequiresApproval = true
		gd.ApprovalReason = "P1 field visit dispatch requires supervisory confirmation per policy_v1"
		return gd
	}

	// Rule C: Low model confidence threshold fallback
	if gd.Confidence < e.minConfidenceThreshold {
		gd.RequiresApproval = true
		gd.ApprovalReason = fmt.Sprintf("Model confidence %.2f below threshold %.2f requires human verification", gd.Confidence, e.minConfidenceThreshold)
		return gd
	}

	gd.RequiresApproval = false
	return gd
}
