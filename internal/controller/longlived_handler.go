package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LongLivedHandler struct {
	Sa  *corev1.ServiceAccount
	Ctx context.Context
	client.Client
	Log logr.Logger
}

func (h *LongLivedHandler) attemptToCreateSecret() error {
	ownerRef := *metav1.NewControllerRef(h.Sa, corev1.SchemeGroupVersion.WithKind("ServiceAccount"))
	*ownerRef.BlockOwnerDeletion = false

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", h.Sa.Name),
			Namespace: h.Sa.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": h.Sa.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	return h.Create(h.Ctx, secret)
}

func (h *LongLivedHandler) Handle() (ctrl.Result, error) {
	h.Log.Info("attempting to create secret for service account", "name", h.Sa.Name, "namespace", h.Sa.Namespace)

	err := h.attemptToCreateSecret()
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			h.Log.Info("secret already exists for the service account, skipping creation", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
			return ctrl.Result{}, nil
		}

		h.Log.Error(err, "failed to create secret for service account", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
	}

	return ctrl.Result{}, err
}
