package controller

import (
	"context"
	"fmt"
	"time"

	oriov1 "github.com/OrRener/service-account-token-operator/api/v1"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *TokenRenewalRequestReconciler) fetchInstance(ctx context.Context, req ctrl.Request) (*oriov1.TokenRenewalRequest, error) {
	instance := &oriov1.TokenRenewalRequest{}

	err := r.Get(ctx, req.NamespacedName, instance)

	return instance, client.IgnoreNotFound(err)
}

func (r *TokenRenewalRequestReconciler) ensureServiceAccountExists(ctx context.Context, trr *oriov1.TokenRenewalRequest) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}

	err := r.Get(ctx, client.ObjectKey{Namespace: trr.Namespace, Name: trr.Spec.ServiceAccountName}, sa)

	if apierrors.IsNotFound(err) {
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      trr.Spec.ServiceAccountName,
				Namespace: trr.Namespace,
			},
		}

		return sa, r.Create(ctx, sa)
	}

	return sa, err
}

func needsRenewal(instance *oriov1.TokenRenewalRequest) bool {
	expiration := instance.Status.TokenExpirationTime
	if expiration == nil {
		return true
	}

	return time.Until(expiration.Time) < time.Minute*30
}

func (r *TokenRenewalRequestReconciler) renewToken(ctx context.Context, sa *corev1.ServiceAccount, instance *oriov1.TokenRenewalRequest) (string, error) {
	tokenReq := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{"https://kubernetes.default.svc"},
			ExpirationSeconds: ptr.To(int64(instance.Spec.RenewalAfter.Seconds())),
		},
	}

	if err := r.SubResource("token").Create(ctx, sa, tokenReq); err != nil {
		return "", err
	}

	token := tokenReq.Status.Token

	ownerRef := *metav1.NewControllerRef(sa, corev1.SchemeGroupVersion.WithKind("ServiceAccount"))
	*ownerRef.BlockOwnerDeletion = false

	secret := &corev1.Secret{
		ObjectMeta: ctrl.ObjectMeta{
			Name:            fmt.Sprintf("%s-token", sa.Name),
			Namespace:       sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	err := r.Update(ctx, secret)
	if apierrors.IsNotFound(err) {
		return token, r.Create(ctx, secret)
	}

	return token, err
}

func (r *TokenRenewalRequestReconciler) calculateRequeuePeriod(instance *oriov1.TokenRenewalRequest) (time.Duration, error) {
	expiration := instance.Status.TokenExpirationTime
	if expiration == nil {
		return 0, fmt.Errorf("unable to find last renewal time in status field")
	}

	return time.Until(expiration.Time) - time.Minute*5, nil
}

func (r *TokenRenewalRequestReconciler) updateGitLabVariable(instance *oriov1.TokenRenewalRequest, token string, ctx context.Context) error {
	secret := &corev1.Secret{}

	err := r.Get(ctx, client.ObjectKey{Name: instance.Spec.GitLabInfo.GitLabTokenSecretRef.Name, Namespace: instance.Namespace}, secret)

	if err != nil {
		return fmt.Errorf("failed to fetch GitLab token secret")
	}

	git, err := gitlab.NewClient(string(secret.Data[instance.Spec.GitLabInfo.GitLabTokenSecretRef.Key]), gitlab.WithBaseURL(instance.Spec.GitLabInfo.GitlabUrl))
	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %v", err)
	}

	projectID := instance.Spec.GitLabInfo.ProjectID
	variableKey := instance.Spec.GitLabInfo.VariableKey
	newValue := token
	variableType := gitlab.VariableTypeValue("env_var")

	opt := &gitlab.CreateProjectVariableOptions{
		Key:          gitlab.Ptr(variableKey),
		Value:        gitlab.Ptr(newValue),
		Protected:    gitlab.Ptr(false),
		Masked:       gitlab.Ptr(true),
		VariableType: &variableType,
	}

	_, _, err = git.ProjectVariables.CreateVariable(projectID, opt)
	if err != nil {
		if respErr, ok := err.(*gitlab.ErrorResponse); ok && respErr.Response.StatusCode == 400 {
			updateOpt := &gitlab.UpdateProjectVariableOptions{
				Value:        gitlab.Ptr(newValue),
				Protected:    gitlab.Ptr(false),
				Masked:       gitlab.Ptr(true),
				VariableType: &variableType,
			}
			_, _, err = git.ProjectVariables.UpdateVariable(projectID, variableKey, updateOpt)
			if err != nil {
				return fmt.Errorf("failed to update variable: %v", err)
			}
			return nil
		}

		return fmt.Errorf("failed to create variable: %v", err)
	}

	return nil
}

func (r *TokenRenewalRequestReconciler) updateInstance(ctx context.Context, instance *oriov1.TokenRenewalRequest,
	message string, success bool, expirationTime, lastRenewalTime *metav1.Time) error {

	instance.Status = oriov1.TokenRenewalRequestStatus{
		Message:             message,
		Success:             success,
		TokenExpirationTime: expirationTime,
		LastRenewalTime:     lastRenewalTime,
	}

	return r.Status().Update(ctx, instance)
}
