package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// IndexingSpec defines chunking, embedding, and vector destination parameters
type IndexingSpec struct {
	ChunkSize      int    `json:"chunkSize,omitempty"`
	ChunkOverlap   int    `json:"chunkOverlap,omitempty"`
	EmbeddingModel string `json:"embeddingModel,omitempty"`
	Collection     string `json:"collection,omitempty"`
}

// EnterpriseKnowledgeSourceSpec defines the desired state of EnterpriseKnowledgeSource
type EnterpriseKnowledgeSourceSpec struct {
	TenantRef            string                      `json:"tenantRef"`
	SourceType           string                      `json:"sourceType"` // s3 | sharepoint | teamcenter | smb
	Endpoint             string                      `json:"endpoint"`
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`
	Indexing             IndexingSpec                `json:"indexing"`
	SyncSchedule         string                      `json:"syncSchedule,omitempty"`
}

// EnterpriseKnowledgeSourceStatus defines the observed state of EnterpriseKnowledgeSource
type EnterpriseKnowledgeSourceStatus struct {
	Phase            string             `json:"phase,omitempty"`
	DocumentsIndexed int                `json:"documentsIndexed,omitempty"`
	LastSyncTime     string             `json:"lastSyncTime,omitempty"`
	VectorDimensions int                `json:"vectorDimensions,omitempty"`
	Conditions       []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Tenant",type="string",JSONPath=".spec.tenantRef"
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.sourceType"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Indexed",type="integer",JSONPath=".status.documentsIndexed"

// EnterpriseKnowledgeSource is the Schema for the enterpriseknowledgesources API
type EnterpriseKnowledgeSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnterpriseKnowledgeSourceSpec   `json:"spec,omitempty"`
	Status EnterpriseKnowledgeSourceStatus `json:"status,omitempty"`
}

// DeepCopyInto copies all properties of this object into another
func (in *EnterpriseKnowledgeSource) DeepCopyInto(out *EnterpriseKnowledgeSource) {
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

// DeepCopyObject returns a generically typed copy of an object
func (in *EnterpriseKnowledgeSource) DeepCopyObject() runtime.Object {
	out := &EnterpriseKnowledgeSource{}
	in.DeepCopyInto(out)
	return out
}

// DeepCopy creates a deep copy
func (in *EnterpriseKnowledgeSource) DeepCopy() *EnterpriseKnowledgeSource {
	if in == nil {
		return nil
	}
	out := new(EnterpriseKnowledgeSource)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true

// EnterpriseKnowledgeSourceList contains a list of EnterpriseKnowledgeSource
type EnterpriseKnowledgeSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnterpriseKnowledgeSource `json:"items"`
}

// DeepCopyInto copies all properties
func (in *EnterpriseKnowledgeSourceList) DeepCopyInto(out *EnterpriseKnowledgeSourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]EnterpriseKnowledgeSource, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (in *EnterpriseKnowledgeSourceList) DeepCopyObject() runtime.Object {
	out := &EnterpriseKnowledgeSourceList{}
	in.DeepCopyInto(out)
	return out
}

func init() {
	SchemeBuilder.Register(&EnterpriseKnowledgeSource{}, &EnterpriseKnowledgeSourceList{})
}
