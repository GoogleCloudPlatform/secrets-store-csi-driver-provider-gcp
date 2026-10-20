// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import "testing"

func TestGkeWorkloadIdentityPool(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    string
	}{
		{
			name: "default universe pool",
			want: "my-project.svc.id.goog",
		},
		{
			name:    "pool overridden from environment",
			envVars: map[string]string{"WORKLOAD_IDENTITY_POOL": "my-project.example.svc.id.goog"},
			want:    "my-project.example.svc.id.goog",
		},
		{
			name:    "empty override falls back to default pool",
			envVars: map[string]string{"WORKLOAD_IDENTITY_POOL": ""},
			want:    "my-project.svc.id.goog",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}
			got, err := gkeWorkloadIdentityPool("my-project")
			if err != nil {
				t.Fatalf("gkeWorkloadIdentityPool() returned an unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("gkeWorkloadIdentityPool() = %v, want: %v", got, tc.want)
			}
		})
	}
}
