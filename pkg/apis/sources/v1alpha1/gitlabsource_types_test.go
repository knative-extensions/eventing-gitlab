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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestGitLabSourceGetConditionSet(t *testing.T) {
	r := &GitLabSource{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestGitLabSourceGetStatus(t *testing.T) {
	status := &duckv1.Status{}
	config := GitLabSource{
		Status: GitLabSourceStatus{
			SourceStatus: duckv1.SourceStatus{Status: *status},
		},
	}

	if !cmp.Equal(config.GetStatus(), status) {
		t.Errorf("GetStatus did not retrieve status. Got=%v Want=%v", config.GetStatus(), status)
	}
}

func TestGitLabSource_GetGroupVersionKind(t *testing.T) {
	gvk := (*GitLabSource)(nil).GetGroupVersionKind()

	const expect = "GitLabSource"

	if gvk.Kind != expect {
		t.Errorf("Expected %q, got %q", expect, gvk.Kind)
	}
}
