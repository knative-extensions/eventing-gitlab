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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventTypes(t *testing.T) {
	definedWebhooks := []string{
		GitLabWebhookPush,               // "push"
		GitLabWebhookMergeRequests,      // "merge_request"
		GitLabWebhookPush,               // repeat a previous item
		GitLabWebhookConfidentialIssues, // / pick webhooks that emit...
		GitLabWebhookIssues,             // \ ...the same event type ("issue")
	}

	expectTypes := []string{
		"dev.knative.sources.gitlab.issue",
		"dev.knative.sources.gitlab.merge_request",
		"dev.knative.sources.gitlab.push",
	}

	testSrc := &GitLabSource{
		Spec: GitLabSourceSpec{
			EventTypes: definedWebhooks,
		},
	}

	assert.Equal(t, expectTypes, testSrc.EventTypes())
}
