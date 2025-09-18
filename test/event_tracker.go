//go:build integration

/*
Copyright 2025 The Knative Authors

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
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CloudEventTracker tracks CloudEvent types seen in the event display logs
type CloudEventTracker struct {
	mu         sync.RWMutex
	eventTypes map[string]bool
}

// NewCloudEventTracker creates a new CloudEventTracker instance
func NewCloudEventTracker() *CloudEventTracker {
	return &CloudEventTracker{
		eventTypes: make(map[string]bool),
	}
}

// AddEventType adds a CloudEvent type to the tracker
func (c *CloudEventTracker) AddEventType(eventType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventTypes[eventType] = true
}

// HasEventType checks if a CloudEvent type has been seen
func (c *CloudEventTracker) HasEventType(eventType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eventTypes[eventType]
}

// GetTrackedEventTypes returns a list of all tracked event types
func (c *CloudEventTracker) GetTrackedEventTypes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var types []string
	for eventType := range c.eventTypes {
		types = append(types, eventType)
	}
	return types
}

// getEventDisplayLogs retrieves the current logs from the event display service
func getEventDisplayLogs(t *testing.T, ctx context.Context, k8sClient kubernetes.Interface, namespace string) string {
	t.Helper()

	// Find the event display pod
	pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "serving.knative.dev/service=gitlab-event-display",
	})
	if err != nil {
		t.Logf("Warning: Failed to list event display pods: %v", err)
		return ""
	}

	if len(pods.Items) == 0 {
		t.Logf("Warning: No event display pods found")
		return ""
	}

	// Get logs from the first pod
	pod := pods.Items[0]
	req := k8sClient.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: "user-container", // Knative serving container name
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		t.Logf("Warning: Failed to stream logs from pod %s: %v", pod.Name, err)
		return ""
	}
	defer logs.Close()

	logBytes, err := io.ReadAll(logs)
	if err != nil {
		t.Logf("Warning: Failed to read logs: %v", err)
		return ""
	}

	return string(logBytes)
}

// streamEventDisplayLogs streams the logs from the event display service and tracks CloudEvent types
func streamEventDisplayLogs(t *testing.T, ctx context.Context, k8sClient kubernetes.Interface, namespace string, tracker *CloudEventTracker) {
	t.Helper()

	var lastLogLength int

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Wait for the event display service to scale up (pods to exist)
			pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "serving.knative.dev/service=gitlab-event-display",
			})
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			// If no pods exist, the service is scaled to zero - wait for scale up
			if len(pods.Items) == 0 {
				time.Sleep(2 * time.Second)
				continue
			}

			// Find a running pod
			var runningPod *corev1.Pod
			for i, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPod = &pods.Items[i]
					break
				}
			}

			// If no running pods, wait for them to start
			if runningPod == nil {
				time.Sleep(2 * time.Second)
				continue
			}

			// Get logs from the running pod
			req := k8sClient.CoreV1().Pods(namespace).GetLogs(runningPod.Name, &corev1.PodLogOptions{
				Container: "user-container", // Knative serving container name
			})

			logs, err := req.Stream(ctx)
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			logBytes, err := io.ReadAll(logs)
			logs.Close()
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			currentLogs := string(logBytes)

			// Only process new log entries since last check
			if len(currentLogs) > lastLogLength {
				newLogs := currentLogs[lastLogLength:]
				lastLogLength = len(currentLogs)

				// Process new log lines for CloudEvent types
				lines := strings.Split(newLogs, "\n")
				for _, line := range lines {
					eventType := extractCloudEventType(t, line)
					if eventType != "" {
						tracker.AddEventType(eventType)
					}
				}
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// extractCloudEventType extracts the CloudEvent type from a log line
func extractCloudEventType(t *testing.T, logLine string) string {
	t.Helper()

	// Look for the line containing "type:" which indicates the event type
	if strings.Contains(logLine, "type:") {
		// Extract the part after "type: "
		parts := strings.Split(logLine, "type: ")
		if len(parts) > 1 {
			// The event type is usually the first word in the next part
			fields := strings.Fields(parts[1])
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}

// getExpectedCloudEventType converts webhook type to expected CloudEvent type
func getExpectedCloudEventType(t *testing.T, webhookType string) string {
	t.Helper()

	eventType := strings.TrimSuffix(webhookType, "-hook")
	return "dev.knative.sources.gitlab." + strings.ReplaceAll(eventType, "-", "_")
}
