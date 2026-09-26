package workflow

type AssetFailureWorkflowInput struct {
	TenantID    string `json:"tenant_id"`
	AssetID     string `json:"asset_id"`
	Symptom     string `json:"symptom"`
	Traceparent string `json:"traceparent,omitempty"`
}

type EvidenceContext struct {
	AssetID            string   `json:"asset_id"`
	TenantID           string   `json:"tenant_id"`
	FailuresLast30Days int      `json:"failures_last_30_days"`
	PLMFindings        string   `json:"plm_findings"`
	SparePartID        string   `json:"spare_part_id"`
	SparePartInStock   bool     `json:"spare_part_in_stock"`
	Recommendation     string   `json:"recommendation"`
	ToolCalls          []string `json:"tool_calls"`
}

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

type ApprovalSignal struct {
	Approved bool   `json:"approved"`
	Approver string `json:"approver,omitempty"`
}

type ReservePartOutput struct {
	ReservationID string `json:"reservation_id"`
	PartID        string `json:"part_id"`
}

type CreateWorkOrderOutput struct {
	WorkOrderID string `json:"work_order_id"`
	AssetID     string `json:"asset_id"`
	Priority    string `json:"priority"`
}

type AssetFailureWorkflowOutput struct {
	WorkflowID    string `json:"workflow_id"`
	Status        string `json:"status"` // "COMPLETED", "REJECTED", "FAILED_COMPENSATED"
	WorkOrderID   string `json:"work_order_id,omitempty"`
	ReservationID string `json:"reservation_id,omitempty"`
	Compensated   bool   `json:"compensated,omitempty"`
}
