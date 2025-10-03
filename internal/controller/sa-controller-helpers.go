package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ServiceAccountReconciler) fetchInstance(ctx context.Context, req ctrl.Request) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	if err := r.Get(ctx, req.NamespacedName, sa); err != nil {
		return nil, err
	}

	return sa, nil
}

func (r *ServiceAccountReconciler) createOrUpdateSecret(ctx context.Context, sa *corev1.ServiceAccount) error {
	ownerRef := *metav1.NewControllerRef(sa, corev1.SchemeGroupVersion.WithKind("ServiceAccount"))
	*ownerRef.BlockOwnerDeletion = false

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", sa.Name),
			Namespace: sa.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	return r.Create(ctx, secret)
}
