/*
Copyright 2020 The Knative Authors.

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

package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"knative.dev/eventing/pkg/reconciler/source"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	sourcesv1alpha1 "knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
	clientset "knative.dev/eventing-gitlab/pkg/client/clientset/versioned"
	listers "knative.dev/eventing-gitlab/pkg/client/listers/sources/v1alpha1"
)

// Reconciler reconciles a GitLabSource object
type Reconciler struct {
	kubeClientSet kubernetes.Interface

	gitlabClientSet clientset.Interface
	gitlabLister    listers.GitLabSourceLister

	servingClientSet servingclientset.Interface
	servingLister    servinglisters.ServiceLister

	receiveAdapterImage string

	sinkResolver *resolver.URIResolver

	loggingContext context.Context

	configs source.ConfigAccessor
}

func (r *Reconciler) ReconcileKind(ctx context.Context, source *sourcesv1alpha1.GitLabSource) reconciler.Event {
	source.Status.InitializeConditions()
	source.Status.ObservedGeneration = source.Generation

	source.Status.CloudEventAttributes = CreateCloudEventAttributes(source.AsEventSource(), source.EventTypes())

	projectName, err := getProjectName(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the project name: " + err.Error())
	}

	hookOptions := projectHookOptions{}
	hookOptions.project = projectName
	hookOptions.id = source.Status.Id

	for _, event := range source.Spec.EventTypes {
		switch event {
		case sourcesv1alpha1.GitLabWebhookConfidentialIssues:
			hookOptions.ConfidentialIssuesEvents = true
		case sourcesv1alpha1.GitLabWebhookConfidentialNote:
			hookOptions.ConfidentialNoteEvents = true
		case sourcesv1alpha1.GitLabWebhookDeployment:
			hookOptions.DeploymentEvents = true
		case sourcesv1alpha1.GitLabWebhookIssues:
			hookOptions.IssuesEvents = true
		case sourcesv1alpha1.GitLabWebhookJob:
			hookOptions.JobEvents = true
		case sourcesv1alpha1.GitLabWebhookMergeRequests:
			hookOptions.MergeRequestsEvents = true
		case sourcesv1alpha1.GitLabWebhookNote:
			hookOptions.NoteEvents = true
		case sourcesv1alpha1.GitLabWebhookPipeline:
			hookOptions.PipelineEvents = true
		case sourcesv1alpha1.GitLabWebhookPush:
			hookOptions.PushEvents = true
		case sourcesv1alpha1.GitLabWebhookTagPush:
			hookOptions.TagPushEvents = true
		case sourcesv1alpha1.GitLabWebhookWikiPage:
			hookOptions.WikiPageEvents = true
		}
	}
	hookOptions.accessToken, err = r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecret("NotFound", "%s", err)
		return err
	}
	hookOptions.secretToken, err = r.secretFrom(ctx, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecret("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSecret()

	sink := source.Spec.Sink.DeepCopy()

	if sink.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if sink.Ref.Namespace == "" {
			//TODO how does this work with deprecated fields
			sink.Ref.Namespace = source.GetNamespace()
		}
	}

	uri, err := r.sinkResolver.URIFromDestinationV1(ctx, *sink, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.getOwnedKnativeService(ctx, source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc = r.generateKnativeServiceObject(source, r.receiveAdapterImage)
			ksvc, err = r.servingClientSet.ServingV1().Services(ksvc.GetNamespace()).Create(ctx, ksvc, metav1.CreateOptions{})
			if err != nil {
				source.Status.MarkNotDeployed("ReceiveAdapterCreationError", "%s", err)
				return err
			}
		} else {
			source.Status.MarkNotDeployed("ReceiveAdapterNotOwned", "%s", err)
			return fmt.Errorf("Failed to verify if knative service is created for the gitlabsource: %v", err)
		}
	}
	if !ksvc.IsReady() {
		source.Status.MarkNotDeployed("ReceiveAdapterNotReady", "Receive adapter Service is not ready")
		return nil
	}
	source.Status.MarkDeployed()

	if ksvc.Status.URL == nil {
		return nil
	}
	hookOptions.url = ksvc.Status.URL.String()
	if source.Spec.SslVerify {
		hookOptions.EnableSSLVerification = true
	}
	baseURL, err := getGitlabBaseURL(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the base url: %v", err)
	}
	gitlabClient := gitlabHookClient{}
	hookID, err := gitlabClient.Create(baseURL, &hookOptions)
	if err != nil {
		source.Status.MarkNoWebhook("WebhookNotConfigured", "%s", err)
		return fmt.Errorf("Failed to create project hook: %v", err)
	}
	source.Status.MarkWebhook()
	source.Status.Id = hookID
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, source *sourcesv1alpha1.GitLabSource) reconciler.Event {
	if source.Status.Id != "" {
		hookOptions := projectHookOptions{}
		projectName, err := getProjectName(source.Spec.ProjectUrl)
		if err != nil {
			return fmt.Errorf("failed to process project url to get the project name: %s", err.Error())
		}
		hookOptions.project = projectName
		hookOptions.id = source.Status.Id
		hookOptions.accessToken, err = r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if err != nil {
			return err
		}
		baseURL, err := getGitlabBaseURL(source.Spec.ProjectUrl)
		if err != nil {
			return fmt.Errorf("failed to process project url to get the base url: %s", err.Error())
		}
		gitlabClient := gitlabHookClient{}
		err = gitlabClient.Delete(baseURL, &hookOptions)
		if err != nil {
			return fmt.Errorf("failed to delete project hook: %s", err.Error())
		}
		source.Status.Id = ""
	}
	return nil
}

func getGitlabBaseURL(projectUrl string) (string, error) {
	u, err := url.Parse(projectUrl)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	baseURL := strings.TrimSuffix(projectUrl, projectName)
	return baseURL, nil
}

func getProjectName(projectUrl string) (string, error) {
	u, err := url.Parse(projectUrl)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	return projectName, nil
}

func (r *Reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret, err := r.kubeClientSet.CoreV1().Secrets(namespace).Get(ctx, secretKeySelector.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf(`key "%s" not found in secret "%s"`, secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
}

func (r *Reconciler) generateKnativeServiceObject(source *sourcesv1alpha1.GitLabSource, receiveAdapterImage string) *servingv1.Service {
	labels := map[string]string{
		"receive-adapter": "gitlab",
	}

	env := append([]corev1.EnvVar{
		{
			Name: "GITLAB_SECRET_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: source.Spec.SecretToken.SecretKeyRef,
			},
		}, {
			Name:  "GITLAB_EVENT_SOURCE",
			Value: source.AsEventSource(),
		}, {
			Name:  "K_SINK",
			Value: source.Status.SinkURI.String(),
		}, {
			Name:  "NAMESPACE",
			Value: source.GetNamespace(),
		}, {
			Name:  "METRICS_DOMAIN",
			Value: "knative.dev/eventing",
		}, {
			Name:  "METRICS_PROMETHEUS_PORT",
			Value: "9092",
		}},
		r.configs.ToEnvVars()...)

	return &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", source.Name),
			Namespace:    source.Namespace,
			Labels:       labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					Spec: servingv1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							ServiceAccountName: source.Spec.ServiceAccountName,
							Containers: []corev1.Container{
								{
									Image: receiveAdapterImage,
									Env:   env,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *Reconciler) getOwnedKnativeService(ctx context.Context, source *sourcesv1alpha1.GitLabSource) (*servingv1.Service, error) {
	list, err := r.servingClientSet.ServingV1().Services(source.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})

	if err != nil {
		return nil, err
	}
	for _, ksvc := range list.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			return &ksvc, nil
		}
	}

	return nil, apierrors.NewNotFound(servingv1.Resource("services"), "")
}

// CreateCloudEventAttributes returns CloudEvent attributes for the event types
// supported by the source.
func CreateCloudEventAttributes(source string, eventTypes []string) []duckv1.CloudEventAttributes {
	ceAttributes := make([]duckv1.CloudEventAttributes, len(eventTypes))

	for i, typ := range eventTypes {
		ceAttributes[i] = duckv1.CloudEventAttributes{
			Type:   typ,
			Source: source,
		}
	}

	return ceAttributes
}
