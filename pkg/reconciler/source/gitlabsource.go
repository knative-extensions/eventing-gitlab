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

package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientv1 "knative.dev/serving/pkg/client/clientset/versioned/typed/serving/v1"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-gitlab/pkg/client/gitlab"
)

// Reconciler reconciles a GitLabSource object
type Reconciler struct {
	gitlabCg gitlab.WebhookClientGetter

	ksvcCli    func(namespace string) servingclientv1.ServiceInterface
	ksvcLister servinglisters.ServiceLister

	receiveAdapterImage string

	sinkResolver *resolver.URIResolver

	loggingContext context.Context

	configs source.ConfigAccessor
}

func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1alpha1.GitLabSource) reconciler.Event {
	src.Status.CloudEventAttributes = CreateCloudEventAttributes(src.AsEventSource(), src.EventTypes())

	sinkURI, err := resolveSinkURL(ctx, r.sinkResolver, src)
	if err != nil {
		src.Status.MarkNoSink()
		return reconciler.NewEvent(corev1.EventTypeWarning,
			"BadSinkURI", "Could not resolve sink URI: %s", err)
	}
	src.Status.MarkSink(sinkURI)

	adapter, err := r.reconcileAdapter(ctx, src)
	if err != nil {
		src.Status.MarkNotDeployed("FailedSync", "Error reconciling receive adapter: %s", err)
		return fmt.Errorf("reconciling receive adapter: %w", err)
	}

	if !adapter.IsReady() {
		src.Status.MarkNotDeployed("NotReady", "Receive adapter Service is not ready")
		return nil
	}
	src.Status.MarkDeployed()

	adapterURL := adapter.Status.URL

	// skip this cycle if the adapter's URL couldn't yet be determined
	if adapterURL == nil {
		return nil
	}

	hookID, err := syncWebhook(ctx, r.gitlabCg, src, adapterURL)
	if err != nil {
		return err
	}

	src.Status.WebhookID = &hookID
	src.Status.MarkWebhook()

	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, src *v1alpha1.GitLabSource) reconciler.Event {
	currentHookID := src.Status.WebhookID

	if currentHookID == nil {
		return nil
	}

	gitlabCli, err := r.gitlabCg.Get(src)
	switch {
	case isSecretNotFound(err):
		// the finalizer is unlikely to recover from missing
		// credentials, so we simply record a warning event and return
		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeWarning, "FailedWebhookDelete",
			"GitLab API token missing while finalizing event source. Ignoring: %s", err)
		return nil

	case isDenied(err):
		// it is unlikely that we recover from auth errors in the
		// finalizer, so we simply record a warning event and return
		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeWarning, "FailedWebhookDelete",
			"Access denied to GitLab API while finalizing event source. Ignoring: %s", err)
		return nil

	case err != nil:
		return reconciler.NewEvent(corev1.EventTypeWarning,
			"ClientError", "Error obtaining GitLab webhook client: %s", err)
	}

	if err := gitlabCli.Delete(*currentHookID); err != nil {
		return err
	}

	src.Status.WebhookID = nil

	return nil
}

// syncWebhook reconciles the GitLab project's webhook with its desired state.
func syncWebhook(ctx context.Context, cg gitlab.WebhookClientGetter,
	src *v1alpha1.GitLabSource, url *apis.URL) (hookID int, err error) {

	cli, err := cg.Get(src)
	switch {
	case isSecretNotFound(err):
		src.Status.MarkNoWebhook("MissingCredentials", "Error obtaining credentials for GitLab API: %s", err)
		return -1, reconciler.NewEvent(corev1.EventTypeWarning,
			"AuthError", "Error obtaining credentials for GitLab API: %s", err)

	case err != nil:
		src.Status.MarkNoWebhook("ClientError", "Error obtaining GitLab webhook client: %s", err)
		// wrap reconciler events to fail (and retry) the reconciliation
		return -1, fmt.Errorf("%w", reconciler.NewEvent(corev1.EventTypeWarning,
			"ClientError", "Error obtaining GitLab webhook client: %s", err))
	}

	currentHookID := src.Status.WebhookID

	if currentHookID == nil {
		hookID, err := cli.Add(src.Spec.EventTypes, url, src.Spec.SSLVerify)
		if err != nil {
			src.Status.MarkNoWebhook("WebhookError", "Error adding webhook: %s", err)
			return -1, fmt.Errorf("%w", reconciler.NewEvent(corev1.EventTypeWarning,
				"WebhookError", "Error adding webhook: %s", err))
		}

		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal,
			"WebHookCreated", "Project webhook created successfully")

		return hookID, nil
	}

	_, err = cli.Get(*currentHookID)
	switch {
	case isHookNotFound(err):
		hookID, err := cli.Add(src.Spec.EventTypes, url, src.Spec.SSLVerify)
		if err != nil {
			src.Status.MarkNoWebhook("WebhookError", "Error adding webhook: %s", err)
			return -1, fmt.Errorf("%w", reconciler.NewEvent(corev1.EventTypeWarning,
				"WebhookError", "Error adding webhook: %s", err))
		}

		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal,
			"WebHookCreated", "Project webhook created successfully")

		return hookID, nil

	case err != nil:
		src.Status.MarkNoWebhook("WebhookError", "Error retrieving webhook: %s", err)
		return -1, fmt.Errorf("%w", reconciler.NewEvent(corev1.EventTypeWarning,
			"WebhookError", "Error retrieving webhook: %s", err))
	}

	err = cli.Edit(*currentHookID, src.Spec.EventTypes, url, src.Spec.SSLVerify)
	if err != nil {
		src.Status.MarkNoWebhook("WebhookError", "Error updating webhook: %s", err)
		return -1, fmt.Errorf("%w", reconciler.NewEvent(corev1.EventTypeWarning,
			"WebhookError", "Error updating webhook: %s", err))
	}

	return *currentHookID, nil
}

// resolveSinkURL resolves the URL of a sink reference.
func resolveSinkURL(ctx context.Context, r *resolver.URIResolver, src *v1alpha1.GitLabSource) (*apis.URL, error) {
	sink := src.Spec.Sink

	if sinkRef := &sink.Ref; *sinkRef != nil && (*sinkRef).Namespace == "" {
		(*sinkRef).Namespace = src.Namespace
	}

	return r.URIFromDestinationV1(ctx, sink, src)
}

// reconcileAdapter reconciles the state of the source's adapter.
func (r *Reconciler) reconcileAdapter(ctx context.Context, src *v1alpha1.GitLabSource) (*servingv1.Service, error) {
	adapter, err := r.getOwnedKnativeService(ctx, src)
	switch {
	case apierrors.IsNotFound(err):
		adapter = r.generateKnativeServiceObject(src, r.receiveAdapterImage)
		adapter, err = r.ksvcCli(src.Namespace).Create(ctx, adapter, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating receive adapter: %w", err)
		}

	case err != nil:
		return nil, fmt.Errorf("searching for existing receive adapter: %w", err)
	}

	return adapter, nil
}

func (r *Reconciler) generateKnativeServiceObject(source *v1alpha1.GitLabSource, receiveAdapterImage string) *servingv1.Service {
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

func (r *Reconciler) getOwnedKnativeService(ctx context.Context, source *v1alpha1.GitLabSource) (*servingv1.Service, error) {
	list, err := r.ksvcCli(source.GetNamespace()).List(ctx, metav1.ListOptions{
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

// isSecretNotFound returns whether the given error indicates that a Kubernetes
// Secret does not exist.
func isSecretNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}

// isHookNotFound returns whether the given error indicates that a GitLab
// project hook does not exist.
func isHookNotFound(err error) bool {
	if glErr := (*gogitlab.ErrorResponse)(nil); errors.As(err, &glErr) {
		return glErr.Response.StatusCode == http.StatusNotFound
	}
	return false
}

// isDenied returns whether the given error indicates that an API request to
// GitLab was denied.
func isDenied(err error) bool {
	if glErr := (*gogitlab.ErrorResponse)(nil); errors.As(err, &glErr) {
		return glErr.Response.StatusCode == http.StatusUnauthorized
	}
	return false
}
