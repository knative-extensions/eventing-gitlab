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
	"fmt"
	"net/url"
	"strings"

	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-gitlab/pkg/secret"
	"knative.dev/pkg/apis"

	gitlab "github.com/xanzy/go-gitlab"
)

// WebhookClient is a client which can interact with the webhook configuration
// of a GitLab project.
type WebhookClient interface {
	Get(hookID int) (*gitlab.ProjectHook, error)
	Add(eventTypes []string, webhookURL *apis.URL, tls bool) (hookID int, err error)
	Edit(hookID int, eventTypes []string, webhookURL *apis.URL, tls bool) error
	Delete(hookID int) error
}

// webhookClient is the default implementation of WebhookClient.
type webhookClient struct {
	// GitLab API client.
	cli *gitlab.Client

	// Name of the GitLab project.
	projectName string

	// Optional user-defined token used to validate requests to webhooks.
	//
	// This value is stored in the client instead of being passed to its
	// methods because, in most cases, the secret token will be stored in
	// the same Kubernetes Secret as the API token. The latter is read from
	// the cluster on every client creation, therefore, we can limit GET
	// requests to the Kubernetes API by reading two values at once.
	secretToken *string
}

// webhookClient implements WebhookClient.
var _ WebhookClient = (*webhookClient)(nil)

// Get adds a new hook to the client's GitLab project.
func (c *webhookClient) Get(hookID int) (*gitlab.ProjectHook, error) {
	hook, _, err := c.cli.Projects.GetProjectHook(c.projectName, hookID)
	if err != nil {
		return nil, fmt.Errorf("getting webhook from project %q: %w", c.projectName, err)
	}

	return hook, nil
}

// Add adds a new hook to the client's GitLab project.
func (c *webhookClient) Add(eventTypes []string, webhookURL *apis.URL, tls bool) (hookID int, err error) {
	hookOptions := gitlab.AddProjectHookOptions{
		URL:                   gitlab.String(webhookURL.String()),
		EnableSSLVerification: &tls,
		Token:                 c.secretToken,
	}

	for _, eventType := range eventTypes {
		switch eventType {
		case v1alpha1.GitLabWebhookConfidentialIssues:
			hookOptions.ConfidentialIssuesEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookConfidentialNote:
			hookOptions.ConfidentialNoteEvents = gitlab.Bool(true)
		// NOTE(antoineco): not supported in this version of xanzy/go-gitlab (v0.39.0)
		//case v1alpha1.GitLabWebhookDeployment:
		//	hookOptions.DeploymentEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookIssues:
			hookOptions.IssuesEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookJob:
			hookOptions.JobEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookMergeRequests:
			hookOptions.MergeRequestsEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookNote:
			hookOptions.NoteEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookPipeline:
			hookOptions.PipelineEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookPush:
			hookOptions.PushEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookTagPush:
			hookOptions.TagPushEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookWikiPage:
			hookOptions.WikiPageEvents = gitlab.Bool(true)
		}
	}

	hook, _, err := c.cli.Projects.AddProjectHook(c.projectName, &hookOptions)
	if err != nil {
		return -1, fmt.Errorf("adding webhook to project %q: %w", c.projectName, err)
	}

	return hook.ID, nil
}

// Edit edits the configuration of a hook in the client's GitLab project.
func (c *webhookClient) Edit(hookID int, eventTypes []string, webhookURL *apis.URL, tls bool) error {
	hookOptions := gitlab.EditProjectHookOptions{
		URL:                   gitlab.String(webhookURL.String()),
		EnableSSLVerification: &tls,
		Token:                 c.secretToken,
	}

	for _, eventType := range eventTypes {
		switch eventType {
		case v1alpha1.GitLabWebhookConfidentialIssues:
			hookOptions.ConfidentialIssuesEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookConfidentialNote:
			hookOptions.ConfidentialNoteEvents = gitlab.Bool(true)
		// NOTE(antoineco): not supported in this version of xanzy/go-gitlab (v0.39.0)
		//case v1alpha1.GitLabWebhookDeployment:
		//	hookOptions.DeploymentEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookIssues:
			hookOptions.IssuesEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookJob:
			hookOptions.JobEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookMergeRequests:
			hookOptions.MergeRequestsEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookNote:
			hookOptions.NoteEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookPipeline:
			hookOptions.PipelineEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookPush:
			hookOptions.PushEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookTagPush:
			hookOptions.TagPushEvents = gitlab.Bool(true)
		case v1alpha1.GitLabWebhookWikiPage:
			hookOptions.WikiPageEvents = gitlab.Bool(true)
		}
	}

	if _, _, err := c.cli.Projects.EditProjectHook(c.projectName, hookID, &hookOptions); err != nil {
		return fmt.Errorf("editing webhook in project %q: %w", c.projectName, err)
	}

	return nil
}

// Delete removes the webhook matching the client's configuration from a GitLab project.
func (c *webhookClient) Delete(hookID int) error {
	if _, err := c.cli.Projects.DeleteProjectHook(c.projectName, hookID); err != nil {
		return fmt.Errorf("deleting webhook from project %q: %w", c.projectName, err)
	}

	return nil
}

// WebhookClientGetter can obtain a GitLab webhook client from a GitLabSource
// API object.
type WebhookClientGetter interface {
	Get(*v1alpha1.GitLabSource) (*webhookClient, error)
}

// NewWebhookClientGetter returns a WebhookClientGetter for the given secrets getter.
func NewWebhookClientGetter(sg NamespacedSecretsGetter) *WebhookClientGetterWithSecretGetter {
	return &WebhookClientGetterWithSecretGetter{
		sg: sg,
	}
}

type NamespacedSecretsGetter func(namespace string) coreclientv1.SecretInterface

// WebhookClientGetterWithSecretGetter gets a GitLab client using static
// credentials retrieved using a Secret getter.
type WebhookClientGetterWithSecretGetter struct {
	sg NamespacedSecretsGetter
}

// WebhookClientGetterWithSecretGetter implements ClientGetter.
var _ WebhookClientGetter = (*WebhookClientGetterWithSecretGetter)(nil)

// Get implements ClientGetter.
func (g *WebhookClientGetterWithSecretGetter) Get(src *v1alpha1.GitLabSource) (*webhookClient, error) {
	baseURL, projectName, err := splitGitLabProjectURL(src.Spec.ProjectUrl)
	if err != nil {
		return nil, fmt.Errorf("reading components from the given project URL: %w", err)
	}

	requestedSecrets, err := secret.NewGetter(g.sg(src.Namespace)).Get(
		src.Spec.AccessToken.SecretKeyRef,
		src.Spec.SecretToken.SecretKeyRef,
	)
	if err != nil {
		return nil, fmt.Errorf("retrieving user-provided GitLab secrets: %w", err)
	}

	apiToken := requestedSecrets[0]
	secretToken := requestedSecrets[1]

	cli, err := gitlab.NewClient(apiToken, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, fmt.Errorf("creating a GitLab client: %w", err)
	}

	var secretTokenPtr *string
	if secretToken != "" {
		secretTokenPtr = &secretToken
	}

	webhookCli := &webhookClient{
		cli:         cli,
		projectName: projectName,
		secretToken: secretTokenPtr,
	}

	return webhookCli, nil
}

// splitGitLabProjectURL returns the base URL and the project name components
// contained in the given GitLab project URL.
// Example: given the project URL "https://gitlab.example.com/myuser/myproject",
// the returned base URL and project name are respectively "https://gitlab.example.com"
// and "myuser/myproject".
func splitGitLabProjectURL(projectURL string) (baseURL, projectName string, err error) {
	u, err := url.Parse(projectURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing project URL %q: %w", projectURL, err)
	}

	projectName = u.Path[1:]
	baseURL = strings.TrimSuffix(projectURL, projectName)

	return baseURL, projectName, nil
}

// WebhookClientGetterFunc allows the use of ordinary functions as WebhookClientGetter.
type WebhookClientGetterFunc func(*v1alpha1.GitLabSource) (*webhookClient, error)

// ClientGetterFunc implements WebhookClientGetter.
var _ WebhookClientGetter = (WebhookClientGetterFunc)(nil)

// Get implements ClientGetter.
func (f WebhookClientGetterFunc) Get(src *v1alpha1.GitLabSource) (*webhookClient, error) {
	return f(src)
}
