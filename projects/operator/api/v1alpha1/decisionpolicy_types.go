package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ApprovalTriggersSpec defines conditions that mandate human supervisor approval
type ApprovalTriggersSpec struct {
	SafetyRiskIn           []string `json:"safetyRiskIn,omitempty"`
	SeverityThreshold      float64  `json:"severityThreshold,omitempty"`
	MinConfidenceThreshold float64  `json:"minConfidenceThreshold,omitempty"`
}

// FinancialLimitsSpec defines financial boundaries for automated operations
type FinancialLimitsSpec struct {
	MaxAutoDispatchCostUSD            float64 `json:"maxAutoDispatchCostUSD,omitempty"`
	RequireApprovalIfPartUnobtainable bool    `json:"requireApprovalIfPartUnobtainable,omitempty"`
}

// PolicyRulesSpec defines the concrete rule sets for decision governance
type PolicyRulesSpec struct {
	ApprovalTriggers ApprovalTriggersSpec `json:"approvalTriggers,omitempty"`
	FinancialLimits  FinancialLimitsSpec  `json:"financialLimits,omitempty"`
}

// DecisionPolicySpec defines the desired state of DecisionPolicy
type DecisionPolicySpec struct {
	Tenant      string          `json:"tenant"`
	Version     string          `json:"version"`
	Description string          `json:"description,omitempty"`
	Rules       PolicyRulesSpec `json:"rules"`
}

// DecisionPolicyStatus defines the observed state of DecisionPolicy
type DecisionPolicyStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Tenant",type="string",JSONPath=".spec.tenant"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"

// DecisionPolicy is the Schema for the decisionpolicies API
type DecisionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DecisionPolicySpec   `json:"spec,omitempty"`
	Status DecisionPolicyStatus `json:"status,omitempty"`
}

// DeepCopyInto copies all properties of this object into another
func (in *DecisionPolicy) DeepCopyInto(out *DecisionPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	if in.Spec.Rules.ApprovalTriggers.SafetyRiskIn != nil {
		out.Spec.Rules.ApprovalTriggers.SafetyRiskIn = make([]string, len(in.Spec.Rules.ApprovalTriggers.SafetyRiskIn))
		copy(out.Spec.Rules.ApprovalTriggers.SafetyRiskIn, in.Spec.Rules.ApprovalTriggers.SafetyRiskIn)
	}
	out.Status = in.Status
	if in.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(in.Status.Conditions))
		copy(out.Status.Conditions, in.Status.Conditions)
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (in *DecisionPolicy) DeepCopyObject() runtime.Object {
	out := &DecisionPolicy{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopy creates a deep copy
func (in *DecisionPolicy) DeepCopy() *DecisionPolicy {
	if in == nil {
		return nil
	}
	out := new(DecisionPolicy)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// DecisionPolicyList contains a list of DecisionPolicy
type DecisionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DecisionPolicy `json:"items"`
}

// DeepCopyInto copies all properties
func (in *DecisionPolicyList) DeepCopyInto(out *DecisionPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]DecisionPolicy, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (in *DecisionPolicyList) DeepCopyObject() runtime.Object {
	out := &DecisionPolicyList{}
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&DecisionPolicy{}, &DecisionPolicyList{})
}
