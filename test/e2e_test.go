//go:build integration

/*
Copyright 2025 The Knative Authors.

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

package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclient "knative.dev/serving/pkg/client/clientset/versioned"

	gitlabclient "knative.dev/eventing-gitlab/pkg/client/clientset/versioned"
)

const (
	// Default namespace for the test
	testNamespace = "default"
	// Expected service name prefix based on the sample GitLabSource
	expectedServicePrefix = "gitlabsource-sample-"
	// Timeout for waiting for resources
	waitTimeout = 5 * time.Minute
	// GitLab webhook header for token validation
	gitlabTokenHeader = "X-Gitlab-Token"
)

func TestE2E_GitLabSourceWebhookIntegration(t *testing.T) {
	// Create Kubernetes client config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig if not running in cluster
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		require.NoError(t, err, "Failed to create Kubernetes client config")
	}

	// Create clients
	servingClient, err := servingclient.NewForConfig(config)
	require.NoError(t, err, "Failed to create Knative Serving client")

	gitlabClient, err := gitlabclient.NewForConfig(config)
	require.NoError(t, err, "Failed to create GitLab client")

	k8sClient, err := kubernetes.NewForConfig(config)
	require.NoError(t, err, "Failed to create Kubernetes client")

	ctx := context.Background()

	// Get the GitLabSource resource
	gitlabSource, err := gitlabClient.SourcesV1alpha1().GitLabSources(testNamespace).Get(ctx, "gitlabsource-sample", metav1.GetOptions{})
	require.NoError(t, err, "Failed to get GitLabSource")

	t.Logf("Found GitLabSource: %s", gitlabSource.Name)
	t.Logf("GitLab Project URL: %s", gitlabSource.Spec.ProjectURL)

	// Get the secret containing the secret token
	secretName := gitlabSource.Spec.SecretToken.SecretKeyRef.Name
	secretKey := gitlabSource.Spec.SecretToken.SecretKeyRef.Key

	secret, err := k8sClient.CoreV1().Secrets(testNamespace).Get(ctx, secretName, metav1.GetOptions{})
	require.NoError(t, err, "Failed to get secret %s", secretName)

	secretToken, exists := secret.Data[secretKey]
	require.True(t, exists, "Secret key %s not found in secret %s", secretKey, secretName)

	t.Logf("Retrieved secret token from secret %s, key %s (length: %d)", secretName, secretKey, len(secretToken))

	// Find the GitLabSource adapter service
	services, err := servingClient.ServingV1().Services(testNamespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err, "Failed to list Knative services")

	var gitlabSourceService *servingv1.Service
	for i, service := range services.Items {
		if strings.HasPrefix(service.Name, expectedServicePrefix) {
			gitlabSourceService = &services.Items[i]
			break
		}
	}

	require.NotNil(t, gitlabSourceService, "Expected to find a Knative service with prefix %s", expectedServicePrefix)
	t.Logf("Found GitLabSource service: %s", gitlabSourceService.Name)

	// Verify the service has the expected labels
	expectedLabels := map[string]string{
		"receive-adapter": "gitlab",
	}
	for key, expectedValue := range expectedLabels {
		actualValue, exists := gitlabSourceService.Labels[key]
		assert.True(t, exists, "Expected label %s to exist", key)
		assert.Equal(t, expectedValue, actualValue, "Expected label %s to have value %s", key, expectedValue)
	}

	// Get the webhook URL
	require.NotNil(t, gitlabSourceService.Status.URL, "Service should have a URL")
	webhookURL := gitlabSourceService.Status.URL.String()
	t.Logf("GitLabSource webhook URL: %s", webhookURL)

	// Start streaming event display logs in background
	tracker := NewCloudEventTracker()
	logCtx, cancelLogs := context.WithCancel(ctx)
	defer cancelLogs()

	go streamEventDisplayLogs(t, logCtx, k8sClient, testNamespace, tracker)

	// Wait a moment for log streaming to start
	time.Sleep(2 * time.Second)

	// Now run the webhook simulation subtests with the discovered endpoint
	t.Run("WebhookSimulation", func(t *testing.T) {
		// Discover all webhook payload files in testdata
		testdataDir := "testdata"
		files, err := os.ReadDir(testdataDir)
		require.NoError(t, err, "Failed to read testdata directory")

		// Filter for webhook payload files
		var webhookFiles []string
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json5") {
				webhookFiles = append(webhookFiles, file.Name())
			}
		}

		require.NotEmpty(t, webhookFiles, "No webhook payload files found in testdata")
		t.Logf("Found %d webhook payload files: %v", len(webhookFiles), webhookFiles)

		// Test each webhook payload file
		for _, filename := range webhookFiles {
			// Extract webhook type from filename (e.g., "push-hook.json5" -> "push-hook")
			webhookType := strings.TrimSuffix(filename, ".json5")

			t.Run(webhookType, func(t *testing.T) {
				// Read the test payload
				payloadFile := testdataDir + "/" + filename
				payload, err := os.ReadFile(payloadFile)
				require.NoError(t, err, "Failed to read test payload file %s", payloadFile)

				// Convert JSON5 to JSON by removing comments
				jsonPayload := removeJSON5Comments(string(payload))

				t.Logf("Sending GitLab %s webhook to %s", webhookType, webhookURL)

				// Create HTTP request
				req, err := http.NewRequest("POST", webhookURL, bytes.NewReader([]byte(jsonPayload)))
				require.NoError(t, err, "Failed to create HTTP request")

				// Set GitLab webhook headers
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "GitLab/test")

				// Set the event type header based on the filename
				eventType := getGitLabEventType(webhookType)
				req.Header.Set("X-Gitlab-Event", eventType)
				req.Header.Set(gitlabTokenHeader, string(secretToken))

				// Send the webhook
				client := &http.Client{Timeout: 30 * time.Second}
				resp, err := client.Do(req)
				require.NoError(t, err, "Failed to send webhook request")
				defer resp.Body.Close()

				// Read response
				respBody, err := io.ReadAll(resp.Body)
				require.NoError(t, err, "Failed to read response body")

				t.Logf("Webhook response status: %d", resp.StatusCode)
				if len(respBody) > 0 {
					t.Logf("Webhook response body: %s", string(respBody))
				}

				// Verify the webhook was accepted (expecting 200-202 range)
				assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
					"Expected successful response for %s webhook, got %d: %s", webhookType, resp.StatusCode, string(respBody))

				// If webhook was accepted, verify that CloudEvents were produced
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					expectedEventType := getExpectedCloudEventType(t, webhookType)
					t.Logf("Waiting for CloudEvent of type: %s", expectedEventType)

					// Use assert.Eventually to wait for the expected event type
					assert.Eventually(t, func() bool {
						return tracker.HasEventType(expectedEventType)
					}, 30*time.Second, 1*time.Second,
						"Expected CloudEvent type %s was not received within timeout", expectedEventType)

					if tracker.HasEventType(expectedEventType) {
						t.Logf("✅ CloudEvent of type %s successfully received", expectedEventType)
					}
				}
			})
		}
	})
}

// removeJSON5Comments removes JSON5 style comments to convert to valid JSON
func removeJSON5Comments(content string) string {
	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		// Remove lines that start with //
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// getGitLabEventType convert "some-hook" to "Some Hook"
func getGitLabEventType(webhookType string) string {
	parts := strings.Split(webhookType, "-")
	for i, part := range parts {
		parts[i] = strings.Title(part)
	}
	return strings.Join(parts, " ")
}
