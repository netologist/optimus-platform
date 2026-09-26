package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// OllayaServiceRef points to the target Ollaya service
type OllayaServiceRef struct {
	Service string `json:"service"`
	Port    int32  `json:"port"`
}

// DecisionModelSpec defines the desired state of DecisionModel
type DecisionModelSpec struct {
	OllayaRef      OllayaServiceRef `json:"ollayaRef"`
	Model          string           `json:"model"`
	KeepAlive      string           `json:"keepAlive,omitempty"`
	Precision      string           `json:"precision,omitempty"`
	QuestionSetRef string           `json:"questionSetRef,omitempty"`
}

// DecisionModelStatus defines the observed state of DecisionModel
type DecisionModelStatus struct {
	Phase        string             `json:"phase,omitempty"`
	Conditions   []metav1.Condition `json:"conditions,omitempty"`
	PulledDigest string             `json:"pulledDigest,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.model"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// DecisionModel is the Schema for the decisionmodels API
type DecisionModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DecisionModelSpec   `json:"spec,omitempty"`
	Status DecisionModelStatus `json:"status,omitempty"`
}

// DeepCopyInto copies all properties
func (in *DecisionModel) DeepCopyInto(out *DecisionModel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	if in.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(in.Status.Conditions))
		copy(out.Status.Conditions, in.Status.Conditions)
	}
}

// DeepCopyObject returns a generically typed copy
func (in *DecisionModel) DeepCopyObject() runtime.Object {
	out := &DecisionModel{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopy creates a deep copy
func (in *DecisionModel) DeepCopy() *DecisionModel {
	if in == nil {
		return nil
	}
	out := new(DecisionModel)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// DecisionModelList contains a list of DecisionModel
type DecisionModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DecisionModel `json:"items"`
}

// DeepCopyInto copies all properties
func (in *DecisionModelList) DeepCopyInto(out *DecisionModelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]DecisionModel, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopyObject returns a generically typed copy
func (in *DecisionModelList) DeepCopyObject() runtime.Object {
	out := &DecisionModelList{}
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&DecisionModel{}, &DecisionModelList{})
}
