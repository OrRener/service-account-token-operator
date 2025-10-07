package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type Handler interface {
	Handle() (ctrl.Result, error)
}

type ServiceAccountReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	LongLivedHandler *LongLivedHandler
}

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("fetching service account instace: ", "name", req.Name, "namespace", req.Namespace)

	sa, err := r.fetchInstance(ctx, req)
	if err != nil {
		log.Error(err, "failed to fetch service account instance")
		return ctrl.Result{}, err
	}

	log.Info("fetched service account instance", "name", sa.Name, "namespace", sa.Namespace)

	handler, err := getHandler(sa, ctx, r.Client, log)
	if err != nil {
		log.Error(err, "failed to parse service account annotation", "name", sa.Name, "namespace", sa.Namespace)
		return ctrl.Result{}, nil
	}

	return handler.Handle()
}

func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasLongLivedAnnotation(e.Object.GetAnnotations()) || hasRenewalAnnotation(e.Object.GetAnnotations())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasLongLivedAnnotation(e.ObjectNew.GetAnnotations()) || hasRenewalAnnotation(e.ObjectNew.GetAnnotations())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, ok := e.Object.GetAnnotations()["or.io/create-secret"]
			return ok
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return hasLongLivedAnnotation(e.Object.GetAnnotations()) || hasRenewalAnnotation(e.Object.GetAnnotations())
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(pred).
		Complete(r)
}
