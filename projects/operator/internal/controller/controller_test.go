package controller

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	platformv1alpha1 "github.com/optimus/projects/operator/api/v1alpha1"
)

func TestDecisionModelReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = platformv1alpha1.AddToScheme(scheme)

	dm := &platformv1alpha1.DecisionModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "laya-default",
			Namespace: "optimus",
		},
		Spec: platformv1alpha1.DecisionModelSpec{
			OllayaRef: platformv1alpha1.OllayaServiceRef{
				Service: "ollaya",
				Port:    11435,
			},
			Model: "laya",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dm).WithStatusSubresource(dm).Build()
	reconciler := &DecisionModelReconciler{Client: client, Scheme: scheme}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "optimus", Name: "laya-default"}}
	res, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error during reconcile: %v", err)
	}
	if res.Requeue {
		t.Fatalf("unexpected requeue")
	}

	var updated platformv1alpha1.DecisionModel
	if err := client.Get(context.Background(), req.NamespacedName, &updated); err != nil {
		t.Fatalf("failed to get reconciled model: %v", err)
	}

	if updated.Status.Phase != "Ready" {
		t.Errorf("expected status phase Ready, got %s", updated.Status.Phase)
	}
	if len(updated.Status.Conditions) == 0 {
		t.Errorf("expected conditions to be set")
	}
}

func TestEnterpriseIntegrationReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = platformv1alpha1.AddToScheme(scheme)

	ei := &platformv1alpha1.EnterpriseIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "acme-eam",
			Namespace: "optimus",
		},
		Spec: platformv1alpha1.EnterpriseIntegrationSpec{
			Tenant:     "acme",
			SystemType: "eam",
			Port:       8081,
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ei).WithStatusSubresource(ei).Build()
	reconciler := &EnterpriseIntegrationReconciler{Client: client, Scheme: scheme}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "optimus", Name: "acme-eam"}}
	_, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error during reconcile: %v", err)
	}

	var updated platformv1alpha1.EnterpriseIntegration
	if err := client.Get(context.Background(), req.NamespacedName, &updated); err != nil {
		t.Fatalf("failed to get reconciled integration: %v", err)
	}

	if updated.Status.Phase != "Ready" {
		t.Errorf("expected phase Ready, got %s", updated.Status.Phase)
	}
	if updated.Status.Endpoint == "" {
		t.Errorf("expected endpoint to be populated")
	}
}

func TestEnterpriseEnvironmentReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = platformv1alpha1.AddToScheme(scheme)

	env := &platformv1alpha1.EnterpriseEnvironment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "optimus",
		},
		Spec: platformv1alpha1.EnterpriseEnvironmentSpec{
			Tenant:  "acme",
			Systems: []string{"eam", "erp"},
			DecisionModelRef: corev1.LocalObjectReference{
				Name: "laya-default",
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(env).WithStatusSubresource(env).Build()
	reconciler := &EnterpriseEnvironmentReconciler{Client: client, Scheme: scheme}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "optimus", Name: "demo"}}
	res, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}
	// Initial reconcile creates child integrations and requeues because they are not yet ready
	if !res.Requeue && res.RequeueAfter == 0 {
		t.Errorf("expected requeue waiting for child integrations")
	}

	// Verify child integrations were created
	var eamIntegration platformv1alpha1.EnterpriseIntegration
	if err := client.Get(context.Background(), types.NamespacedName{Namespace: "optimus", Name: "acme-eam"}, &eamIntegration); err != nil {
		t.Errorf("expected child integration acme-eam to be created: %v", err)
	}
}
func TestEnterpriseKnowledgeSourceReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = platformv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	ks := &platformv1alpha1.EnterpriseKnowledgeSource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "optimus",
			Name:      "plm-cooling-manuals",
		},
		Spec: platformv1alpha1.EnterpriseKnowledgeSourceSpec{
			TenantRef:  "acme",
			SourceType: "s3",
			Endpoint:   "https://s3.eu-west-1.amazonaws.com/acme-plm-docs/pumps",
			CredentialsSecretRef: corev1.LocalObjectReference{
				Name: "acme-plm-s3-credentials",
			},
			Indexing: platformv1alpha1.IndexingSpec{
				ChunkSize:      512,
				ChunkOverlap:   64,
				EmbeddingModel: "text-embedding-3-small",
				Collection:     "acme_plm_manuals",
			},
			SyncSchedule: "0 2 * * *",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ks).WithStatusSubresource(ks).Build()
	reconciler := &EnterpriseKnowledgeSourceReconciler{Client: client, Scheme: scheme}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "optimus", Name: "plm-cooling-manuals"}}
	res, err := reconciler.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}
	if res.Requeue {
		t.Errorf("unexpected requeue")
	}

	// 1. Verify status updated to Ready
	var updated platformv1alpha1.EnterpriseKnowledgeSource
	if err := client.Get(context.Background(), req.NamespacedName, &updated); err != nil {
		t.Fatalf("failed to get reconciled knowledge source: %v", err)
	}
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected status Ready, got %s", updated.Status.Phase)
	}
	if updated.Status.VectorDimensions != 384 {
		t.Errorf("expected vector dimensions 384, got %d", updated.Status.VectorDimensions)
	}

	// 2. Verify ingestion worker Job was dispatched
	var syncJob batchv1.Job
	jobKey := types.NamespacedName{Namespace: "optimus", Name: "knowledge-sync-plm-cooling-manuals"}
	if err := client.Get(context.Background(), jobKey, &syncJob); err != nil {
		t.Fatalf("expected ingestion worker Job to be created: %v", err)
	}
	if len(syncJob.Spec.Template.Spec.Containers) == 0 {
		t.Fatalf("expected at least one container in ingestion worker Job")
	}
	container := syncJob.Spec.Template.Spec.Containers[0]
	if container.Name != "ingestion-worker" {
		t.Errorf("expected container name ingestion-worker, got %s", container.Name)
	}
}
