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

package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"gopkg.in/go-playground/webhooks.v5/gitlab"

	"knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/adapter/v2"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

const (
	secretToken = "gitlabsecret"
	projectURL  = "http://gitlab.example.com/myuser/myproject"
)

// testCase holds a single row of our GitLabSource table tests
type testCase struct {
	// name is a descriptive name for this test suitable as a first argument to t.Run()
	name string

	// wantErr is the expected error returned in the server's response
	wantErr error

	// which status code server should return
	statusCode int

	// payload contains the GitLab event payload
	payload interface{}

	// eventType is the GitLab event type
	eventType gitlab.Event
}

var testCases = []testCase{
	{
		name:       "valid comment",
		payload:    gitlab.CommentEventPayload{},
		eventType:  gitlab.CommentEvents,
		statusCode: 202,
	}, {
		name:       "valid issues",
		payload:    gitlab.IssueEventPayload{},
		eventType:  gitlab.IssuesEvents,
		statusCode: 202,
	}, {
		name:       "valid push",
		payload:    gitlab.PushEventPayload{},
		eventType:  gitlab.PushEvents,
		statusCode: 202,
	}, {
		name:       "valid tag event",
		payload:    gitlab.TagEventPayload{},
		eventType:  gitlab.TagEvents,
		statusCode: 202,
	}, {
		name:       "valid confidential issue event",
		payload:    gitlab.ConfidentialIssueEventPayload{},
		eventType:  gitlab.ConfidentialIssuesEvents,
		statusCode: 202,
	}, {
		name:       "valid merge request event",
		payload:    gitlab.MergeRequestEventPayload{},
		eventType:  gitlab.MergeRequestEvents,
		statusCode: 202,
	}, {
		name:       "valid wiki page event",
		payload:    gitlab.WikiPageEventPayload{},
		eventType:  gitlab.WikiPageEvents,
		statusCode: 202,
	}, {
		name:       "valid pipeline event",
		payload:    gitlab.PipelineEventPayload{},
		eventType:  gitlab.PipelineEvents,
		statusCode: 202,
	}, {
		name:       "valid build event",
		payload:    gitlab.BuildEventPayload{},
		eventType:  gitlab.BuildEvents,
		statusCode: 202,
	}, {
		name:       "invalid nil payload",
		payload:    nil,
		eventType:  gitlab.Event("Invalid Hook"),
		wantErr:    gitlab.ErrEventNotFound,
		statusCode: 400,
	}, {
		name:       "invalid empty eventType",
		wantErr:    gitlab.ErrMissingGitLabEventHeader,
		statusCode: 400,
	},
}

func TestGracefulShutdown(t *testing.T) {
	ce := adaptertest.NewTestClient()
	ra := newTestAdapter(t, ce)
	ctx, cancel := context.WithCancel(context.TODO())

	go func() {
		t.Logf("starting webhook server")
		err := ra.Start(ctx)
		if err != nil {
			t.Error(err)
		}
		cancel()
	}()

	cancel()
	<-ctx.Done()
}

func TestServer(t *testing.T) {
	for _, tc := range testCases {
		ce := adaptertest.NewTestClient()
		ra := newTestAdapter(t, ce)
		hook, err := gitlab.New(gitlab.Options.Secret(ra.secretToken))
		if err != nil {
			t.Error(err)
		}
		router := ra.newRouter(hook)
		server := httptest.NewServer(router)
		defer server.Close()

		t.Run(tc.name, tc.runner(t, server.URL, ce))
	}
}

func newTestAdapter(t *testing.T, ce cloudevents.Client) *gitLabReceiveAdapter {
	env := envConfig{
		EnvConfig: adapter.EnvConfig{
			Namespace: "default",
		},
		EnvSecret:   secretToken,
		EventSource: projectURL,
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)
	logger := zap.NewExample().Sugar()
	ctx = logging.WithLogger(ctx, logger)

	return NewAdapter(ctx, &env, ce).(*gitLabReceiveAdapter)
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, url string, ceClient *adaptertest.TestCloudEventsClient) func(*testing.T) {
	return func(t *testing.T) {
		reqBody, _ := json.Marshal(tc.payload)
		req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("X-Gitlab-Event", string(tc.eventType))
		req.Header.Set("X-Gitlab-Token", string(secretToken))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != tc.statusCode {
			t.Errorf("Unexpected status code: %s", resp.Status)
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}

		tc.validateResponse(t, string(respBody))

		tc.validateAcceptedPayload(t, ceClient, tc.statusCode)
	}
}

func (tc *testCase) validateAcceptedPayload(t *testing.T, ce *adaptertest.TestCloudEventsClient, httpCode int) {
	sentEvents := ce.Sent()

	if httpCode/100 != 2 {
		require.Len(t, sentEvents, 0, "Event sent despite the non-success HTTP code")
		return
	}
	require.Len(t, sentEvents, 1, "More than 1 event was sent in reaction to the webhooks's message")

	expectCEType := v1alpha1.GitLabEventType(gitlabEventHeaderToEventType(string(tc.eventType)))
	expectCESource := projectURL
	expectCEExt := string(tc.eventType)
	expectData, err := json.Marshal(tc.payload)
	require.NoError(t, err, "Unable to serialize GitLab payload")

	sentEvent := ce.Sent()[0]

	assert.Equal(t, expectCEType, sentEvent.Type(),
		"CloudEvent type doesn't match the webhook's event header")
	assert.Equal(t, expectCESource, sentEvent.Source(),
		"CloudEvent source doesn't match the project's URL")
	assert.Equal(t, expectCEExt, sentEvent.Extensions()[glHeaderEventCEAttr],
		"CloudEvent extension doesn't match the match the webhook's event header")
	assert.Equal(t, expectData, sentEvent.Data(),
		"CloudEvent data differs from original payload")
}

func (tc *testCase) validateResponse(t *testing.T, body string) {
	if tc.wantErr != nil {
		assert.EqualError(t, tc.wantErr, body)
		return
	}
	assert.Empty(t, body)
}

func TestGitLabEventHeaderToEventType(t *testing.T) {
	testCases := map[string]struct {
		input  string
		expect string
	}{
		"bad format": {
			input:  "Missing The Suffix",
			expect: "",
		},
		"good format": {
			input:  "Legit Event Type Hook",
			expect: "legit_event_type",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(*testing.T) {
			assert.Equal(t, tc.expect, gitlabEventHeaderToEventType(tc.input))
		})
	}
}
