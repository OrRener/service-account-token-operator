package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RenewalHandler struct {
	Sa  *corev1.ServiceAccount
	Ctx context.Context
	client.Client
	Log          logr.Logger
	RenewalAfter time.Duration
}

func (h *RenewalHandler) needsRenewal() (bool, error) {
	val, ok := h.Sa.Annotations["or.io/token-expiration"]
	if !ok {
		return true, nil
	}

	expiration, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return false, err
	}

	return time.Until(expiration) < time.Minute*30, nil
}

func (h *RenewalHandler) renewToken() error {
	tokenReq := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{"https://kubernetes.default.svc"},
			ExpirationSeconds: ptr.To(int64(h.RenewalAfter.Seconds())),
		},
	}

	if err := h.SubResource("token").Create(h.Ctx, h.Sa, tokenReq); err != nil {
		return err
	}

	ownerRef := *metav1.NewControllerRef(h.Sa, corev1.SchemeGroupVersion.WithKind("ServiceAccount"))
	*ownerRef.BlockOwnerDeletion = false

	secret := &corev1.Secret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:            fmt.Sprintf("%s-token", h.Sa.Name),
			Namespace:       h.Sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": h.Sa.Name,
			},
		},
		Data: map[string][]byte{
			"token": []byte(tokenReq.Status.Token),
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	err := h.Update(h.Ctx, secret)
	if apierrors.IsNotFound(err) {
		return h.Create(h.Ctx, secret)
	}

	return err
}

func (h *RenewalHandler) updateServiceAccountAnnotation() error {
	h.Sa.Annotations["or.io/last-renewal"] = time.Now().UTC().Format(time.RFC3339)
	h.Sa.Annotations["or.io/token-expiration"] = time.Now().UTC().Add(h.RenewalAfter).Format(time.RFC3339)

	return h.Update(h.Ctx, h.Sa)
}

func (h *RenewalHandler) calculateRequeuePeriod() (time.Duration, error) {
	val, ok := h.Sa.Annotations["or.io/token-expiration"]
	if !ok {
		return 0, fmt.Errorf("service account %s does not have last-renewal annotation", h.Sa.Name)
	}

	expiration, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return 0, err
	}

	return time.Until(expiration) - time.Minute*5, nil
}

func (h *RenewalHandler) Handle() (ctrl.Result, error) {
	needsRewnwal, err := h.needsRenewal()
	if err != nil {
		h.Log.Error(err, "failed to determine if service account needs renewal", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
		return ctrl.Result{}, err
	}

	if needsRewnwal {
		h.Log.Info("service account token needs renewal, updating last-renewal annotation", "name", h.Sa.Name, "namespace", h.Sa.Namespace)

		if err := h.renewToken(); err != nil {
			h.Log.Error(err, "failed to renew token for service account", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
			return ctrl.Result{}, err
		}

		if err := h.updateServiceAccountAnnotation(); err != nil {
			h.Log.Error(err, "failed to update service account annotation", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
			return ctrl.Result{}, err
		}

		h.Log.Info("successfully renewed token for service account", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
	}

	requeuePeriod, err := h.calculateRequeuePeriod()
	if err != nil {
		h.Log.Error(err, "failed to calculate requeue period for service account", "name", h.Sa.Name, "namespace", h.Sa.Namespace)
		return ctrl.Result{}, err
	}

	h.Log.Info("requeuing reconciliation for service account", "name", h.Sa.Name, "namespace", h.Sa.Namespace, "after", requeuePeriod.String())

	return ctrl.Result{RequeueAfter: requeuePeriod}, nil
}
