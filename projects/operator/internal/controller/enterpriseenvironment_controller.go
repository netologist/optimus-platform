package controller

import (
	"context"
	"fmt"
	"time"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/optimus/projects/operator/api/v1alpha1"
)

// EnterpriseEnvironmentReconciler reconciles a EnterpriseEnvironment object
type EnterpriseEnvironmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseenvironments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseenvironments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseintegrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.optimus.dev,resources=decisionmodels,verbs=get;list;watch

func (r *EnterpriseEnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("enterpriseenvironment", req.NamespacedName)

	var env platformv1alpha1.EnterpriseEnvironment
	if err := r.Get(ctx, req.NamespacedName, &env); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling EnterpriseEnvironment", "tenant", env.Spec.Tenant, "systems", env.Spec.Systems)

	total := len(env.Spec.Systems)
	readyCount := 0

	// 1. Reconcile child EnterpriseIntegrations for each system
	for _, sys := range env.Spec.Systems {
		integrationName := fmt.Sprintf("%s-%s", env.Spec.Tenant, sys)
		var integration platformv1alpha1.EnterpriseIntegration
		err := r.Get(ctx, client.ObjectKey{Namespace: env.Namespace, Name: integrationName}, &integration)
		if apierrors.IsNotFound(err) {
			// Create child integration
			newIntegration := platformv1alpha1.EnterpriseIntegration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      integrationName,
					Namespace: env.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "optimus-operator",
						"platform.optimus.dev/tenant":  env.Spec.Tenant,
					},
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(&env, platformv1alpha1.GroupVersion.WithKind("EnterpriseEnvironment")),
					},
				},
				Spec: platformv1alpha1.EnterpriseIntegrationSpec{
					Tenant:       env.Spec.Tenant,
					SystemType:   sys,
					Port:         8080,
					Capabilities: []string{fmt.Sprintf("%s.query", sys)},
				},
			}
			if err := r.Create(ctx, &newIntegration); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create EnterpriseIntegration %s: %w", integrationName, err)
			}
			logger.Info("Created child EnterpriseIntegration", "name", integrationName)
		} else if err != nil {
			return ctrl.Result{}, err
		} else {
			if integration.Status.Phase == "Ready" {
				readyCount++
			}
		}
	}

	// 2. Check DecisionModel reference readiness
	modelReady := false
	if env.Spec.DecisionModelRef.Name != "" {
		var dm platformv1alpha1.DecisionModel
		err := r.Get(ctx, client.ObjectKey{Namespace: env.Namespace, Name: env.Spec.DecisionModelRef.Name}, &dm)
		if err == nil && dm.Status.Phase == "Ready" {
			modelReady = true
		}
	} else {
		modelReady = true
	}

	// 3. Update Status
	env.Status.TotalIntegrations = total
	env.Status.IntegrationsReady = readyCount

	isReady := (readyCount == total) && modelReady
	conditionStatus := metav1.ConditionFalse
	reason := "IntegrationsPending"
	message := fmt.Sprintf("%d of %d integrations ready, modelReady=%t", readyCount, total, modelReady)

	if isReady {
		env.Status.Phase = "Ready"
		conditionStatus = metav1.ConditionTrue
		reason = "AllSystemsReady"
		message = "All enterprise integrations and decision model are healthy"
	} else {
		env.Status.Phase = "Provisioning"
	}

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             conditionStatus,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Upsert Ready condition
	updatedConditions := make([]metav1.Condition, 0, len(env.Status.Conditions)+1)
	for _, c := range env.Status.Conditions {
		if c.Type != "Ready" {
			updatedConditions = append(updatedConditions, c)
		}
	}
	updatedConditions = append(updatedConditions, readyCondition)
	env.Status.Conditions = updatedConditions

	if err := r.Status().Update(ctx, &env); err != nil {
		return ctrl.Result{}, err
	}

	if !isReady {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnterpriseEnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.EnterpriseEnvironment{}).
		Owns(&platformv1alpha1.EnterpriseIntegration{}).
		Complete(r)
}
