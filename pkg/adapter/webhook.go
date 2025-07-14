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

// ref: https://gitlab.com/gitlab-org/api/client-go/-/blob/main/examples/webhook.go?ref_type=heads

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var (
	ErrMissingGitLabEventHeader      = errors.New("missing X-Gitlab-Event Header")
	ErrEventNotSpecifiedToParse      = errors.New("event not defined to be parsed")
	ErrReadingfRequestBody           = errors.New("error reading request body")
	ErrGitLabTokenVerificationFailed = errors.New("token validation failed")
	ErrCouldNotParseWebhookEvent     = errors.New("could parse the webhook event")
	ErrCouldNotHandleEvent           = errors.New("error handling the event")
)

type EventSender func(payload interface{}, header http.Header) error

// webhook is a HTTP Handler for Gitlab Webhook events.
type webhook struct {
	Secret      string
	EventSender EventSender
}

// webhookExample shows how to create a Webhook server to parse Gitlab events.
func NewWebhookHandler(secret string, sender EventSender) webhook {
	wh := webhook{
		Secret:      secret,
		EventSender: sender,
	}

	return wh
}

// ServeHTTP tries to parse Gitlab events sent and calls handle function
// with the successfully parsed events.
func (hook webhook) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	event, err := hook.parse(request)
	if err != nil {
		writer.WriteHeader(400)
		fmt.Fprintf(writer, "%v: %v", ErrCouldNotParseWebhookEvent, err)
		return
	}

	// Handle the event before we return.
	if err := hook.EventSender(event, request.Header); err != nil {
		writer.WriteHeader(500)
		fmt.Fprintf(writer, "%v: %v", ErrCouldNotHandleEvent, err)
		return
	}

	// Write a response when were done.
	writer.WriteHeader(202)
}

// parse verifies and parses the events specified in the request and
// returns the parsed event or an error.
func (hook webhook) parse(r *http.Request) (any, error) {
	defer func() {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			log.Printf("could discard request body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			log.Printf("could not close request body: %v", err)
		}
	}()

	if r.Method != http.MethodPost {
		return nil, errors.New("invalid HTTP Method")
	}

	// If we have a secret set, we should check if the request matches it.
	if len(hook.Secret) > 0 {
		signature := r.Header.Get("X-Gitlab-Token")
		if signature != hook.Secret {
			return nil, ErrGitLabTokenVerificationFailed
		}
	}

	event := r.Header.Get("X-Gitlab-Event")
	if strings.TrimSpace(event) == "" {
		return nil, ErrMissingGitLabEventHeader
	}

	eventType := gitlab.EventType(event)

	payload, err := io.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, ErrReadingfRequestBody
	}

	return gitlab.ParseWebhook(eventType, payload)
}
