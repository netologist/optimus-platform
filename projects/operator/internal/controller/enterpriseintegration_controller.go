package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/optimus/projects/operator/api/v1alpha1"
)

// EnterpriseIntegrationReconciler reconciles a EnterpriseIntegration object
type EnterpriseIntegrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseintegrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseintegrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *EnterpriseIntegrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("enterpriseintegration", req.NamespacedName)

	var integration platformv1alpha1.EnterpriseIntegration
	if err := r.Get(ctx, req.NamespacedName, &integration); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling EnterpriseIntegration", "tenant", integration.Spec.Tenant, "systemType", integration.Spec.SystemType)

	endpoint := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", integration.Name, integration.Namespace, integration.Spec.Port)
	integration.Status.Endpoint = endpoint
	integration.Status.Phase = "Ready"

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "ServiceReconciled",
		Message:            fmt.Sprintf("Mock endpoint active at %s", endpoint),
	}

	updatedConditions := make([]metav1.Condition, 0, len(integration.Status.Conditions)+1)
	for _, c := range integration.Status.Conditions {
		if c.Type != "Ready" {
			updatedConditions = append(updatedConditions, c)
		}
	}
	updatedConditions = append(updatedConditions, readyCondition)
	integration.Status.Conditions = updatedConditions

	if err := r.Status().Update(ctx, &integration); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EnterpriseIntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.EnterpriseIntegration{}).
		Complete(r)
}
