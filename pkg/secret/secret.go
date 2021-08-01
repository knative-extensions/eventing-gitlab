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

// Package secret contains utilities for consuming secret values from various
// data sources.
package secret

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Secrets is list of secret values.
type Secrets []string

// Getter can obtain secrets.
type Getter interface {
	// Get returns exactly one secret value per input.
	Get(...*corev1.SecretKeySelector) (Secrets, error)
}

// NewGetter returns a Getter for the given namespaced Secret client interface.
func NewGetter(cli coreclientv1.SecretInterface) *GetterWithClientset {
	return &GetterWithClientset{
		cli: cli,
	}
}

// GetterWithClientset gets Kubernetes secrets using a namespaced Secret client
// interface.
type GetterWithClientset struct {
	cli coreclientv1.SecretInterface
}

// GetterWithClientset implements Getter.
var _ Getter = (*GetterWithClientset)(nil)

// Get implements Getter.
func (g *GetterWithClientset) Get(refs ...*corev1.SecretKeySelector) (Secrets, error) {
	s := make(Secrets, 0, len(refs))

	// cache Secret objects by name between iterations to avoid multiple
	// round trips to the Kubernetes API for the same Secret object.
	secretCache := make(map[string]*corev1.Secret)

	for _, ref := range refs {
		var val string

		if ref != nil {
			var secr *corev1.Secret
			var err error

			if secretCache != nil && secretCache[ref.Name] != nil {
				secr = secretCache[ref.Name]
			} else {
				secr, err = g.cli.Get(context.Background(), ref.Name, metav1.GetOptions{})
				if err != nil {
					return nil, fmt.Errorf("getting Secret %q from cluster: %w", ref.Name, err)
				}

				secretCache[ref.Name] = secr
			}

			val = string(secr.Data[ref.Key])
		}

		s = append(s, val)
	}

	return s, nil
}

// GetterFunc allows the use of ordinary functions as Getter.
type GetterFunc func(...*corev1.SecretKeySelector) (Secrets, error)

// GetterFunc implements Getter.
var _ Getter = (GetterFunc)(nil)

// Get implements Getter.
func (f GetterFunc) Get(refs ...*corev1.SecretKeySelector) (Secrets, error) {
	return f(refs...)
}
