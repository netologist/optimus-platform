package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// AISpec defines AI features for the environment
type AISpec struct {
	RAG bool `json:"rag"`
	MCP bool `json:"mcp"`
}

// EnterpriseEnvironmentSpec defines the desired state of EnterpriseEnvironment
type EnterpriseEnvironmentSpec struct {
	Tenant           string                   `json:"tenant"`
	Systems          []string                 `json:"systems"`
	DecisionModelRef corev1.LocalObjectReference `json:"decisionModelRef"`
	AI               AISpec                   `json:"ai"`
}

// EnterpriseEnvironmentStatus defines the observed state of EnterpriseEnvironment
type EnterpriseEnvironmentStatus struct {
	Phase             string             `json:"phase,omitempty"`
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
	IntegrationsReady int                `json:"integrationsReady,omitempty"`
	TotalIntegrations int                `json:"totalIntegrations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Tenant",type="string",JSONPath=".spec.tenant"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// EnterpriseEnvironment is the Schema for the enterpriseenvironments API
type EnterpriseEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnterpriseEnvironmentSpec   `json:"spec,omitempty"`
	Status EnterpriseEnvironmentStatus `json:"status,omitempty"`
}

// DeepCopyInto copies all properties of this object into another
func (in *EnterpriseEnvironment) DeepCopyInto(out *EnterpriseEnvironment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	if in.Spec.Systems != nil {
		out.Spec.Systems = make([]string, len(in.Spec.Systems))
		copy(out.Spec.Systems, in.Spec.Systems)
	}
	out.Status = in.Status
	if in.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(in.Status.Conditions))
		copy(out.Status.Conditions, in.Status.Conditions)
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (in *EnterpriseEnvironment) DeepCopyObject() runtime.Object {
	out := &EnterpriseEnvironment{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopy creates a deep copy
func (in *EnterpriseEnvironment) DeepCopy() *EnterpriseEnvironment {
	if in == nil {
		return nil
	}
	out := new(EnterpriseEnvironment)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// EnterpriseEnvironmentList contains a list of EnterpriseEnvironment
type EnterpriseEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnterpriseEnvironment `json:"items"`
}

// DeepCopyInto copies all properties
func (in *EnterpriseEnvironmentList) DeepCopyInto(out *EnterpriseEnvironmentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]EnterpriseEnvironment, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (in *EnterpriseEnvironmentList) DeepCopyObject() runtime.Object {
	out := &EnterpriseEnvironmentList{}
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&EnterpriseEnvironment{}, &EnterpriseEnvironmentList{})
}
