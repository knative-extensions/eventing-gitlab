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
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	// GitLabSourceConditionReady has status True when the
	// GitLabSource is ready to send events.
	GitLabSourceConditionReady = apis.ConditionReady

	// GitLabSourceConditionSinkProvided has status True when the
	// GitLabSource has been configured with a sink target.
	GitLabSourceConditionSinkProvided apis.ConditionType = "SinkProvided"

	// GitLabSourceConditionWebhookConfigured has a status True when the
	// GitLabSource has been configured with a webhook.
	GitLabSourceConditionWebhookConfigured apis.ConditionType = "WebhookConfigured"

	// GitLabSourceConditionDeployed has status True when the
	// GitLabSource's receive adapter has been successfully deployed.
	GitLabSourceConditionDeployed apis.ConditionType = "Deployed"
)

var gitLabSourceCondSet = apis.NewLivingConditionSet(
	GitLabSourceConditionSinkProvided,
	GitLabSourceConditionDeployed,
	GitLabSourceConditionWebhookConfigured,
)

// GetGroupVersionKind returns a GitLabSource GVK. Implements the kmeta.OwnerRefable interface.
func (*GitLabSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GitLabSource")
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*GitLabSource) GetConditionSet() apis.ConditionSet {
	return gitLabSourceCondSet
}

// GetStatus retrieves the duck status for this resource. Implements the KRShaped interface.
func (g *GitLabSource) GetStatus() *duckv1.Status {
	return &g.Status.Status
}

// MarkSink sets the SinkProvided condition to True using the given URI.
func (s *GitLabSourceStatus) MarkSink(uri *apis.URL) {
	s.SinkURI = uri
	if uri == nil {
		gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionSinkProvided,
			"EmptySinkURI", "The sink has no URI")
		return
	}
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionSinkProvided)
}

// MarkNoSink sets the SinkProvided condition to False.
func (s *GitLabSourceStatus) MarkNoSink() {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionSinkProvided,
		"SinkNotFound", "The sink does not exist or its URI is not set")
}

// MarkWebhook sets the WebhookConfigured condition to True.
func (s *GitLabSourceStatus) MarkWebhook() {
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionWebhookConfigured)
}

// MarkNoWebhook sets the WebhookConfigured condition to False with the given reason and message.
func (s *GitLabSourceStatus) MarkNoWebhook(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionWebhookConfigured, reason, messageFormat, messageA...)
}

// MarkWebhook sets the Deployed condition to True.
func (s *GitLabSourceStatus) MarkDeployed() {
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionDeployed)
}

// MarkNotDeployed sets the Deployed condition to False with the given reason and message.
func (s *GitLabSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionDeployed, reason, messageFormat, messageA...)
}

// String prepended to GitLab event types to make them fully-qualified.
const eventPrefixGitLab = "dev.knative.sources.gitlab."

// Types of events emitted by a GitLabSource.
// The chosen format and case matches the "object_kind" attribute contained in
// payloads sent by GitLab's webhooks.
// https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#events
const (
	GitLabEventTypeBuild        = "build"
	GitLabEventTypeDeployment   = "deployment"
	GitLabEventTypeIssue        = "issue"
	GitLabEventTypeMergeRequest = "merge_request"
	GitLabEventTypeNote         = "note"
	GitLabEventTypePipeline     = "pipeline"
	GitLabEventTypePush         = "push"
	GitLabEventTypeTagPush      = "tag_push"
	GitLabEventTypeWikiPage     = "wiki_page"
)

// Types of webhooks that can be enabled on a GitLab project.
// https://docs.gitlab.com/ee/api/projects.html#add-project-hook
const (
	GitLabWebhookConfidentialIssues = "confidential_issues_events"
	GitLabWebhookConfidentialNote   = "confidential_note_events"
	GitLabWebhookDeployment         = "deployment_events"
	GitLabWebhookIssues             = "issues_events"
	GitLabWebhookJob                = "job_events"
	GitLabWebhookMergeRequests      = "merge_requests_events"
	GitLabWebhookNote               = "note_events"
	GitLabWebhookPipeline           = "pipeline_events"
	GitLabWebhookPush               = "push_events"
	GitLabWebhookTagPush            = "tag_push_events"
	GitLabWebhookWikiPage           = "wiki_page_events"
)

// GitLabEventType returns a GitLab event type in a format suitable for usage
// as a CloudEvent type attribute.
func GitLabEventType(eventType string) string {
	return eventPrefixGitLab + eventType
}

// EventTypes returns the types of events emitted by the source, sorted in
// increasing lexical order.
func (s *GitLabSource) EventTypes() []string {
	// Some webhooks emit the same event type, so we use a map as an
	// intermediate store to avoid duplicates in the returned slice.
	uniqueTypes := make(map[string]struct{}, len(s.Spec.EventTypes))

	for _, hook := range s.Spec.EventTypes {
		uniqueTypes[eventTypeForWebhook(hook)] = struct{}{}
	}

	types := make([]string, 0, len(uniqueTypes))

	for typ := range uniqueTypes {
		types = append(types, GitLabEventType(typ))
	}
	sort.Strings(types)

	return types
}

// eventTypeForWebhook returns the type of event emitted by a given GitLab
// webhook.
func eventTypeForWebhook(webhookName string) string {
	eventTypesByWebhook := map[string]string{
		GitLabWebhookConfidentialIssues: GitLabEventTypeIssue,
		GitLabWebhookConfidentialNote:   GitLabEventTypeNote,
		GitLabWebhookDeployment:         GitLabEventTypeDeployment,
		GitLabWebhookIssues:             GitLabEventTypeIssue,
		GitLabWebhookJob:                GitLabEventTypeBuild,
		GitLabWebhookMergeRequests:      GitLabEventTypeMergeRequest,
		GitLabWebhookNote:               GitLabEventTypeNote,
		GitLabWebhookPipeline:           GitLabEventTypePipeline,
		GitLabWebhookPush:               GitLabEventTypePush,
		GitLabWebhookTagPush:            GitLabEventTypeTagPush,
		GitLabWebhookWikiPage:           GitLabEventTypeWikiPage,
	}

	return eventTypesByWebhook[webhookName]
}

// AsEventSource returns a unique reference to the source suitable for use as a
// CloudEvent source attribute.
func (s *GitLabSource) AsEventSource() string {
	return s.Spec.ProjectURL
}
