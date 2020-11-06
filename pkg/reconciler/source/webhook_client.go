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
	"fmt"
	"net/http"
	"strconv"

	gitlab "github.com/xanzy/go-gitlab"
)

type projectHookOptions struct {
	accessToken           string
	secretToken           string
	project               string
	id                    string
	url                   string
	EnableSSLVerification bool

	ConfidentialIssuesEvents bool
	ConfidentialNoteEvents   bool
	DeploymentEvents         bool
	IssuesEvents             bool
	JobEvents                bool
	MergeRequestsEvents      bool
	NoteEvents               bool
	PipelineEvents           bool
	PushEvents               bool
	TagPushEvents            bool
	WikiPageEvents           bool
}

type gitlabHookClient struct{}

func (client gitlabHookClient) Create(baseURL string, options *projectHookOptions) (string, error) {
	glClient, err := gitlab.NewClient(options.accessToken, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	if options.id != "" {
		hookID, err := strconv.Atoi(options.id)
		if err != nil {
			return "", fmt.Errorf("failed to convert hook id to int: %s", err.Error())
		}
		projhooks, resp, err := glClient.Projects.ListProjectHooks(options.project,
			&gitlab.ListProjectHooksOptions{
				// Max number of hook per project
				PerPage: 100,
			}, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list project hooks for project %q due to an error: %s", options.project, err.Error())
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("project hooks list unexpected status: %s", resp.Status)
		}
		for _, hook := range projhooks {
			if hook.ID == hookID {
				return options.id, nil
			}
		}
	}

	hookOptions := gitlab.AddProjectHookOptions{
		Token:                 &options.secretToken,
		URL:                   &options.url,
		EnableSSLVerification: &options.EnableSSLVerification,

		ConfidentialIssuesEvents: &options.ConfidentialIssuesEvents,
		ConfidentialNoteEvents:   &options.ConfidentialNoteEvents,
		IssuesEvents:             &options.IssuesEvents,
		JobEvents:                &options.JobEvents,
		MergeRequestsEvents:      &options.MergeRequestsEvents,
		NoteEvents:               &options.NoteEvents,
		PipelineEvents:           &options.PipelineEvents,
		PushEvents:               &options.PushEvents,
		TagPushEvents:            &options.TagPushEvents,
		WikiPageEvents:           &options.WikiPageEvents,
		// NOTE(antoineco): not supported in this version of xanzy/go-gitlab
		//DeploymentEvents: &options.DeploymentEvents,
	}

	hook, _, err := glClient.Projects.AddProjectHook(options.project, &hookOptions, nil)
	if err != nil {
		return "", fmt.Errorf("failed to add webhook to the project %q due to an error: %s ", options.project, err.Error())
	}

	return strconv.Itoa(hook.ID), nil
}

func (client gitlabHookClient) Delete(baseURL string, options *projectHookOptions) error {
	if options.id != "" {
		hookID, err := strconv.Atoi(options.id)
		if err != nil {
			return fmt.Errorf("failed to convert hook id to int: " + err.Error())
		}
		glClient, err := gitlab.NewClient(options.accessToken, gitlab.WithBaseURL(baseURL))
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		projhooks, _, err := glClient.Projects.ListProjectHooks(options.project, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list project hooks for project: " + options.project)
		}
		for _, hook := range projhooks {
			if hook.ID == hookID {
				_, err = glClient.Projects.DeleteProjectHook(options.project, hookID, nil)
				if err != nil {
					return fmt.Errorf("Failed to delete project hook: " + err.Error())
				}
			}
		}
	}

	return nil
}
