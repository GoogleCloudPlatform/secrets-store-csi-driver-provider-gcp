// Copyright 2025 Google LLC
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

import (
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/vars"
)

func TestFormatWorkloadIdentityPool(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		want      string
	}{
		{
			name:      "standard GDU project",
			projectID: "my-gdu-project",
			want:      "my-gdu-project.svc.id.goog",
		},
		{
			name:      "TPC project with universe prefix",
			projectID: "tpczero-system:pragyass-tpc",
			want:      "pragyass-tpc.tpczero-system.svc.id.goog",
		},
		{
			name:      "sovereign partner TPC project",
			projectID: "s3ns:my-project",
			want:      "my-project.s3ns.svc.id.goog",
		},
		{
			name:      "already suffixed workload pool",
			projectID: "my-project.svc.id.goog",
			want:      "my-project.svc.id.goog",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatWorkloadIdentityPool(tc.projectID); got != tc.want {
				t.Errorf("formatWorkloadIdentityPool(%q) = %q, want %q", tc.projectID, got, tc.want)
			}
		})
	}
}

func TestGkeWorkloadIdentityEnvVars(t *testing.T) {
	t.Cleanup(func() {
		vars.SetUniverseDomain(vars.DefaultUniverseDomain)
	})

	tests := []struct {
		name           string
		universeDomain string
		envVars        map[string]string
		wantPool       string
		wantProvider   string
	}{
		{
			name:           "GDU cluster via env vars",
			universeDomain: vars.DefaultUniverseDomain,
			envVars: map[string]string{
				"PROJECT":          "my-gdu-project",
				"CLUSTER_LOCATION": "us-central1",
				"CLUSTER_NAME":     "gdu-cluster",
			},
			wantPool:     "my-gdu-project.svc.id.goog",
			wantProvider: "https://container.googleapis.com/v1/projects/my-gdu-project/locations/us-central1/clusters/gdu-cluster",
		},
		{
			name:           "TPC cluster via env vars with LOCATION fallback",
			universeDomain: "apis-tpczero.goog",
			envVars: map[string]string{
				"PROJECT":      "tpczero-system:pragyass-tpc",
				"LOCATION":     "u-us-prp1",
				"CLUSTER_NAME": "tpc-cluster",
			},
			wantPool:     "pragyass-tpc.tpczero-system.svc.id.goog",
			wantProvider: "https://container.apis-tpczero.goog/v1/projects/tpczero-system:pragyass-tpc/locations/u-us-prp1/clusters/tpc-cluster",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PROJECT", "")
			t.Setenv("CLUSTER_LOCATION", "")
			t.Setenv("LOCATION", "")
			t.Setenv("CLUSTER_NAME", "")
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}
			vars.SetUniverseDomain(tc.universeDomain)

			c := &Client{}
			gotPool, gotProvider, err := c.gkeWorkloadIdentity(context.Background(), &config.MountConfig{})
			if err != nil {
				t.Fatalf("gkeWorkloadIdentity() unexpected error: %v", err)
			}
			if gotPool != tc.wantPool {
				t.Errorf("gkeWorkloadIdentity() pool = %q, want %q", gotPool, tc.wantPool)
			}
			if gotProvider != tc.wantProvider {
				t.Errorf("gkeWorkloadIdentity() provider = %q, want %q", gotProvider, tc.wantProvider)
			}
		})
	}
}
