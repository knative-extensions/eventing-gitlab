/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

var (
	_ apis.Validatable   = (*GitLabSource)(nil)
	_ apis.Defaultable   = (*GitLabSource)(nil)
	_ kmeta.OwnerRefable = (*GitLabSource)(nil)
	_ duckv1.KRShaped    = (*GitLabSource)(nil)
)

// GitLabSourceSpec defines the desired state of GitLabSource
// +kubebuilder:categories=all,knative,eventing,sources
type GitLabSourceSpec struct {
	duckv1.SourceSpec `json:",inline"`

	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GitLabSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// ProjectURL is the url of the GitLab project for which we are interested
	// to receive events from.
	// Examples:
	//   https://gitlab.com/gitlab-org/gitlab-foss
	// +optional
	ProjectURL string `json:"projectUrl,omitempty"`

	// GroupURL is the url of the GitLab group for which we are interested
	// to receive events from.
	// Examples:
	//   https://gitlab.com/gitlab-org/gitlab-foss
	// +optional
	GroupURL string `json:"groupUrl,omitempty"`

	// List of webhooks to enable on the selected GitLab project.
	// Those correspond to the attributes enumerated at
	// https://docs.gitlab.com/ee/api/projects.html#add-project-hook
	EventTypes []string `json:"eventTypes"`

	// AccessToken is the Kubernetes secret containing the GitLab
	// access token
	AccessToken SecretValueFromSource `json:"accessToken"`

	// SecretToken is the Kubernetes secret containing the GitLab
	// secret token
	SecretToken SecretValueFromSource `json:"secretToken"`

	// SSLVerify if true configure webhook so the ssl verification is done when triggering the hook
	SSLVerify bool `json:"sslverify,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// GitLabSourceStatus defines the observed state of GitLabSource
type GitLabSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`

	// WebhookID of the project hook registered with GitLab
	WebhookID *int `json:"webhookID,omitempty"`
}

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabSource is the Schema for the gitlabsources API.
type GitLabSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitLabSourceSpec   `json:"spec,omitempty"`
	Status GitLabSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabSourceList contains a list of GitLabSource.
type GitLabSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitLabSource `json:"items"`
}
