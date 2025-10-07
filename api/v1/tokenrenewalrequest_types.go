package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GitLabTokenSecretRef struct {
	Name string `json:"name"`
	// +optional
	// +kubebuilder:default:=token
	Key string `json:"key"`
}

type GitLabInfo struct {
	GitlabUrl            string                `json:"gitlabUrl"`
	ProjectID            int                   `json:"projectID"`
	VariableKey          string                `json:"variableKey"`
	GitLabTokenSecretRef *GitLabTokenSecretRef `json:"gitLabTokenSecretRef"`
}

// TokenRenewalRequestSpec defines the desired state of TokenRenewalRequest
type TokenRenewalRequestSpec struct {
	// ServiceAccountName is the name of the service account to renew the token for
	// +required
	ServiceAccountName string `json:"serviceAccountName"`

	// RenewalAfter is the duration after which the token should be renewed (seconds/minutes/hours)
	// +required
	RenewalAfter metav1.Duration `json:"renewalAfter"`

	// GitLabInfo is the information about the varaible in GitLab to update
	// +optional
	GitLabInfo *GitLabInfo `json:"gitLabInfo,omitempty,omitzero"`
}

// TokenRenewalRequestStatus defines the observed state of TokenRenewalRequest.
type TokenRenewalRequestStatus struct {
	// LastRenewalTime is the last time the token was renewed
	// +optional
	LastRenewalTime *metav1.Time `json:"lastRenewalTime,omitempty,omitzero"`

	// TokenExpirationTime is the expiration time of the current token
	// +optional
	TokenExpirationTime *metav1.Time `json:"tokenExpirationTime,omitempty,omitzero"`

	// Success indicates whether the last renewal was successful
	// +optional
	Success bool `json:"success,omitempty,omitzero"`

	// Message is a human-readable message indicating details about the last renewal
	// +optional
	Message string `json:"message,omitempty,omitzero"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TokenRenewalRequest is the Schema for the tokenrenewalrequests API
type TokenRenewalRequest struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of TokenRenewalRequest
	// +required
	Spec TokenRenewalRequestSpec `json:"spec"`

	// status defines the observed state of TokenRenewalRequest
	// +optional
	Status TokenRenewalRequestStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// TokenRenewalRequestList contains a list of TokenRenewalRequest
type TokenRenewalRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TokenRenewalRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TokenRenewalRequest{}, &TokenRenewalRequestList{})
}
