package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	oriov1 "github.com/OrRener/service-account-token-operator/api/v1"
)

type TokenRenewalRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=or.io.or.io,resources=tokenrenewalrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=or.io.or.io,resources=tokenrenewalrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=or.io.or.io,resources=tokenrenewalrequests/finalizers,verbs=update

func (r *TokenRenewalRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("fetching TokenRenewalRequest instance:")

	instance, err := r.fetchInstance(ctx, req)
	if err != nil {
		log.Error(err, "failed to fetch TokenRenewalRequest instance")
		return ctrl.Result{}, err
	}

	if instance == nil {
		log.Info("TokenRenewalRequest instance not found, might have been deleted")
		return ctrl.Result{}, nil
	}

	log.Info("successfully fetched TokenRenewalRequest instance")

	log.Info("ensuring the serviceAccount mentioned under ServiceAccountName exists, this may create the serviceAccount")

	sa, err := r.ensureServiceAccountExists(ctx, instance)
	if err != nil {
		log.Error(err, "failed to ensure the ServiceAccount mentioned in the ServiceAccountName field exists")

		if err := r.updateInstance(ctx, instance, err.Error(), false, nil, nil); err != nil {
			log.Error(err, "failed to update status of instance:")
		}

		return ctrl.Result{}, err
	}

	needsRenew := needsRenewal(instance)

	if needsRenew {
		log.Info("token needs renewal, renewing...")

		token, err := r.renewToken(ctx, sa, instance)
		if err != nil {
			log.Error(err, "failed to renew token for service account")

			if err := r.updateInstance(ctx, instance, err.Error(), false, nil, nil); err != nil {
				log.Error(err, "failed to update status of instance:")
			}

			return ctrl.Result{}, err
		}

		if instance.Spec.GitLabInfo != nil {
			log.Info("updating GitLab variable with new token")

			if err := r.updateGitLabVariable(instance, token, ctx); err != nil {
				log.Error(err, "failed to update GitLab variable with the new token")

				if err := r.updateInstance(ctx, instance, err.Error(), false, nil, nil); err != nil {
					log.Error(err, "failed to update status of instance:")
				}

				return ctrl.Result{}, err
			}
		}

		log.Info(fmt.Sprintf("successfully renewed token for service account, the new token is populated at the %s-token secret", instance.Spec.ServiceAccountName))

		expirationTime := metav1.NewTime(time.Now().Add(instance.Spec.RenewalAfter.Duration))
		renewalTime := metav1.NewTime(time.Now())

		if err := r.updateInstance(ctx, instance, "token is valid", true, &expirationTime, &renewalTime); err != nil {
			log.Error(err, "failed to update status of instance:")

			return ctrl.Result{}, err
		}
	} else {
		log.Info("token doesn't need renewal, requeuing the request to a time the token will need renewal")
	}

	requeuePeriod, err := r.calculateRequeuePeriod(instance)
	if err != nil {
		log.Error(err, "failed to calculate requeue period")

		if err := r.updateInstance(ctx, instance, err.Error(), false, nil, nil); err != nil {
			log.Error(err, "failed to update status of instance:")
		}

		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeuePeriod}, nil
}

func (r *TokenRenewalRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&oriov1.TokenRenewalRequest{}).
		Named("tokenrenewalrequest").
		WithEventFilter(pred).
		Complete(r)
}
