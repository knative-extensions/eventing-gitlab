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
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"gopkg.in/go-playground/webhooks.v5/gitlab"

	sourcesv1alpha1 "knative.dev/eventing-gitlab/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"
)

const (
	glHeaderEvent       = "X-Gitlab-Event"
	glHeaderEventCEAttr = "comgitlabevent"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable containing Gitlab secret token
	EnvSecret string `envconfig:"GITLAB_SECRET_TOKEN" required:"true"`
	// Port to listen incoming connections
	Port string `envconfig:"PORT" default:"8080"`
	// Name of the event source to set as source attribute on emitted CloudEvents.
	EventSource string `envconfig:"GITLAB_EVENT_SOURCE" required:"true"`
}

// gitLabReceiveAdapter converts incoming GitLab webhook events to
// CloudEvents and then sends them to the specified Sink
type gitLabReceiveAdapter struct {
	logger      *zap.SugaredLogger
	client      cloudevents.Client
	eventSource string
	secretToken string
	port        string
}

// NewEnvConfig function reads env variables defined in envConfig structure and
// returns accessor interface
func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// NewAdapter returns the instance of gitLabReceiveAdapter that implements adapter.Adapter interface
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	return &gitLabReceiveAdapter{
		logger:      logger,
		client:      ceClient,
		eventSource: env.EventSource,
		secretToken: env.EnvSecret,
		port:        env.Port,
	}
}

// Start implements adapter.Adapter
func (ra *gitLabReceiveAdapter) Start(ctx context.Context) error {
	return ra.start(ctx.Done())
}

func (ra *gitLabReceiveAdapter) start(stopCh <-chan struct{}) error {
	hook, err := gitlab.New(gitlab.Options.Secret(ra.secretToken))
	if err != nil {
		return fmt.Errorf("cannot create gitlab hook: %v", err)
	}

	server := &http.Server{
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		Addr:              ":" + ra.port,
		Handler:           ra.newRouter(hook),
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		gracefulShutdown(server, ra.logger, stopCh)
	}()

	ra.logger.Info("Server is ready to handle requests at ", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", server.Addr, err)
	}

	wg.Wait()
	ra.logger.Info("Server stopped")
	return nil
}

func gracefulShutdown(server *http.Server, logger *zap.SugaredLogger, stopCh <-chan struct{}) {
	<-stopCh
	logger.Info("Server is shutting down...")

	// Try to graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Could not gracefully shutdown the server: ", err)
	}
}

func (ra *gitLabReceiveAdapter) newRouter(hook *gitlab.Webhook) *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r,
			gitlab.PushEvents,
			gitlab.TagEvents,
			gitlab.IssuesEvents,
			gitlab.ConfidentialIssuesEvents,
			gitlab.CommentEvents,
			gitlab.MergeRequestEvents,
			gitlab.WikiPageEvents,
			gitlab.PipelineEvents,
			gitlab.BuildEvents,
		)
		if err != nil {
			ra.logger.Error("Hook parser error: ", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		if err := ra.handleEvent(payload, r.Header); err != nil {
			ra.logger.Error("Event handler error: ", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		ra.logger.Debug("Event processed")
		w.WriteHeader(http.StatusAccepted)
	})

	return router
}

func (ra *gitLabReceiveAdapter) handleEvent(payload interface{}, header http.Header) error {
	eventHeader := header.Get(glHeaderEvent)
	eventType := gitlabEventHeaderToEventType(eventHeader)
	if eventType == "" {
		return fmt.Errorf("invalid webhook event type %s", eventHeader)
	}

	ceType := sourcesv1alpha1.GitLabEventType(eventType)

	extensions := map[string]interface{}{
		glHeaderEventCEAttr: eventHeader,
	}

	return ra.postMessage(payload, ra.eventSource, ceType, extensions)
}

func (ra *gitLabReceiveAdapter) postMessage(payload interface{}, source, eventType string,
	extensions map[string]interface{}) error {

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource(source)

	for ext, val := range extensions {
		event.SetExtension(ext, val)
	}

	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	if result := ra.client.Send(context.Background(), event); !cloudevents.IsACK(result) {
		return result
	}
	return nil
}

// gitlabEventHeaderToEventType transforms the value of a X-Gitlab-Event header
// for a webhook request into the corresponding CloudEvent event type.
// The value of the header follows the format "Some Type Hook", which we
// convert to snake_case after trimming the "Hook" qualifier.
// https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#events
func gitlabEventHeaderToEventType(header string) string {
	const headerSuffix = " Hook"

	if !strings.HasSuffix(header, headerSuffix) {
		return ""
	}

	return strings.ToLower(strings.ReplaceAll(
		header[:len(header)-len(headerSuffix)],
		" ", "_",
	))
}
