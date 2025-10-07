package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func hasLongLivedAnnotation(annotations map[string]string) bool {
	_, ok := annotations["or.io/create-secret"]
	return ok
}

func hasRenewalAnnotation(annotations map[string]string) bool {
	_, ok := annotations["or.io/renew-after"]
	return ok
}

func getRenewalPeriod(annotations map[string]string) (time.Duration, error) {
	dur, err := time.ParseDuration(annotations["or.io/renew-after"])
	if err != nil {
		return 0, err
	}

	if dur < time.Hour*24 {
		return 0, fmt.Errorf("renewal period must be at least 24h, got %s", dur.String())
	}

	return dur, nil

}

func getHandler(sa *corev1.ServiceAccount, ctx context.Context, runtimeClient client.Client, log logr.Logger) (Handler, error) {
	annotations := sa.Annotations

	if hasLongLivedAnnotation(annotations) {
		return &LongLivedHandler{Sa: sa, Ctx: ctx, Log: log, Client: runtimeClient}, nil
	}

	if hasRenewalAnnotation(annotations) {
		renewalPeriod, err := getRenewalPeriod(annotations)
		if err != nil {
			return nil, err
		}
		return &RenewalHandler{Sa: sa, Ctx: ctx, Log: log, Client: runtimeClient, RenewalAfter: renewalPeriod}, nil
	}

	return nil, fmt.Errorf("no handler found for service account %s/%s, this might mean that the annotation is not set correctly", sa.Namespace, sa.Name)
}

func (r *ServiceAccountReconciler) fetchInstance(ctx context.Context, req ctrl.Request) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}

	if err := r.Get(ctx, req.NamespacedName, sa); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	return sa, nil
}
