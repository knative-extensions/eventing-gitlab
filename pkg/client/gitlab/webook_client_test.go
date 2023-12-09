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
					ProjectURL: "https://gitlab.com/project",
				},
			},
			ExpectedType: &projectWebhookClient{},
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
