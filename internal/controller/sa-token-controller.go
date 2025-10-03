package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("fetching service account instace: ", "name", req.Name, "namespace", req.Namespace)
	sa, err := r.fetchInstance(ctx, req)
	if err != nil {
		log.Error(err, "failed to fetch service account instance")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("attempting to create secret for service account: ", "name", sa.Name, "namespace", sa.Namespace)
	if err = r.createOrUpdateSecret(ctx, sa); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Error(err, "failed to create or update secret for the service account")
			return ctrl.Result{}, err
		} else {
			log.Info("secret for service account already exists, skipping creation", "name", sa.Name, "namespace", sa.Namespace)
		}
	}

	log.Info("completed reconciliation for service account: ", "name", sa.Name, "namespace", sa.Namespace)

	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.GetAnnotations()["or.io/create-secret"]
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, ok := e.ObjectNew.GetAnnotations()["or.io/create-secret"]
			return ok
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, ok := e.Object.GetAnnotations()["or.io/create-secret"]
			return ok
		},
		GenericFunc: func(e event.GenericEvent) bool {
			_, ok := e.Object.GetAnnotations()["or.io/create-secret"]
			return ok
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(pred).
		Complete(r)
}
