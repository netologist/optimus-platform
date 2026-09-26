package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EnterpriseIntegrationSpec defines the desired state of EnterpriseIntegration
type EnterpriseIntegrationSpec struct {
	Tenant       string   `json:"tenant"`
	SystemType   string   `json:"systemType"`
	Port         int32    `json:"port"`
	Capabilities []string `json:"capabilities"`
	SecretRef    string   `json:"secretRef,omitempty"`
}

// EnterpriseIntegrationStatus defines the observed state of EnterpriseIntegration
type EnterpriseIntegrationStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Endpoint   string             `json:"endpoint,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Tenant",type="string",JSONPath=".spec.tenant"
// +kubebuilder:printcolumn:name="System",type="string",JSONPath=".spec.systemType"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.endpoint"

// EnterpriseIntegration is the Schema for the enterpriseintegrations API
type EnterpriseIntegration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnterpriseIntegrationSpec   `json:"spec,omitempty"`
	Status EnterpriseIntegrationStatus `json:"status,omitempty"`
}

// DeepCopyInto copies all properties
func (in *EnterpriseIntegration) DeepCopyInto(out *EnterpriseIntegration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	if in.Spec.Capabilities != nil {
		out.Spec.Capabilities = make([]string, len(in.Spec.Capabilities))
		copy(out.Spec.Capabilities, in.Spec.Capabilities)
	}
	out.Status = in.Status
	if in.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(in.Status.Conditions))
		copy(out.Status.Conditions, in.Status.Conditions)
	}
}

// DeepCopyObject returns a generically typed copy
func (in *EnterpriseIntegration) DeepCopyObject() runtime.Object {
	out := &EnterpriseIntegration{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopy creates a deep copy
func (in *EnterpriseIntegration) DeepCopy() *EnterpriseIntegration {
	if in == nil {
		return nil
	}
	out := new(EnterpriseIntegration)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// EnterpriseIntegrationList contains a list of EnterpriseIntegration
type EnterpriseIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnterpriseIntegration `json:"items"`
}

// DeepCopyInto copies all properties
func (in *EnterpriseIntegrationList) DeepCopyInto(out *EnterpriseIntegrationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]EnterpriseIntegration, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopyObject returns a generically typed copy
func (in *EnterpriseIntegrationList) DeepCopyObject() runtime.Object {
	out := &EnterpriseIntegrationList{}
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&EnterpriseIntegration{}, &EnterpriseIntegrationList{})
}
