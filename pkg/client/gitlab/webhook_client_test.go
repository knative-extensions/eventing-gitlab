/*
Copyright 2023 The Knative Authors

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
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
)

func TestWebhookClientGetterReturnsExpectedWebhookClient(t *testing.T) {
	testCases := map[string]struct {
		Spec         *v1alpha1.GitLabSource
		ExpectedType interface{}
	}{
		"GitLab ProjectURL": {
			Spec: &v1alpha1.GitLabSource{
				Spec: v1alpha1.GitLabSourceSpec{
					ProjectURL: "https://gitlab.com/project",
				},
			},
			ExpectedType: &projectWebhookClient{},
		},
		"GitLab GroupURL": {
			Spec: &v1alpha1.GitLabSource{
				Spec: v1alpha1.GitLabSourceSpec{
					GroupURL: "https://gitlab.com/group",
				},
			},
			ExpectedType: &groupWebhookClient{},
		},
	}

	fakeclient := fake.NewSimpleClientset()

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			gitlabCg := NewWebhookClientGetter(fakeclient.CoreV1().Secrets)
			gitlabCi, err := gitlabCg.Get(tc.Spec)

			require.NoError(t, err)
			require.IsType(t, tc.ExpectedType, gitlabCi)
		})
	}
}

func TestWebhookClientGetterReturnsErrorWithoutProjectOrGroupURL(t *testing.T) {
	spec := &v1alpha1.GitLabSource{
		Spec: v1alpha1.GitLabSourceSpec{},
	}

	fakeclient := fake.NewSimpleClientset()

	gitlabCg := NewWebhookClientGetter(fakeclient.CoreV1().Secrets)
	gitlabCi, err := gitlabCg.Get(spec)

	require.Errorf(t, err, "reading the project or group url from the gitlab source spec: project or group url not found in gitlab source spec")
	require.Nil(t, gitlabCi)
}
