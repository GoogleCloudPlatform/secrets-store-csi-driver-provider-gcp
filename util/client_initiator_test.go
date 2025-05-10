// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"context"
	"testing"

	"google.golang.org/api/option"
)

func TestGetRegionalSecretManagerClient(t *testing.T) {
	ctx := context.Background()
	baseOpts := []option.ClientOption{option.WithoutAuthentication()}

	tests := []struct {
		name          string
		region        string
		clientOptions []option.ClientOption
		wantNil       bool
		wantEndpoint  string
	}{
		{
			name:          "valid region",
			region:        "us-central1",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "secretmanager.us-central1.rep.googleapis.com:443",
		},
		{
			name:          "another valid region",
			region:        "europe-west1",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "secretmanager.europe-west1.rep.googleapis.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := GetRegionalSecretManagerClient(ctx, tt.region, tt.clientOptions)
			if client == nil {
				t.Errorf("GetRegionalSecretManagerClient() with region '%s' = nil, want non-nil client", tt.region)
			} else {
				if err := client.Close(); err != nil {
					t.Logf("Error closing client for region '%s': %v", tt.region, err)
				}
			}
			conn := client.Connection()
			if conn == nil {
				t.Errorf("Expected non nil connection for region '%s', got nil", tt.region)
			}
			actualEndpoint := conn.Target()
			if actualEndpoint != tt.wantEndpoint {
				t.Errorf("GetRegionalSecretManagerClient() with region '%s' has endpoint '%s', want '%s'", tt.region, actualEndpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestGetRegionalParameterManagerClient(t *testing.T) {
	ctx := context.Background()
	baseOpts := []option.ClientOption{option.WithoutAuthentication()}

	tests := []struct {
		name          string
		region        string
		clientOptions []option.ClientOption
		wantNil       bool
		wantEndpoint  string
	}{
		{
			name:          "valid region",
			region:        "europe-west3",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "parametermanager.europe-west3.rep.googleapis.com:443",
		},
		{
			name:          "another valid region",
			region:        "us-east7",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "parametermanager.us-east7.rep.googleapis.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := GetRegionalParameterManagerClient(ctx, tt.region, tt.clientOptions)

			if client == nil {
				t.Errorf("GetRegionalParameterManagerClient() with region '%s' = nil, want non-nil client", tt.region)
			} else {
				if err := client.Close(); err != nil {
					t.Logf("Error closing client for region '%s': %v", tt.region, err)
				}
			}
			conn := client.Connection()
			if conn == nil {
				t.Errorf("Expected non nil connection for region '%s', got nil", tt.region)
			}
			actualEndpoint := conn.Target()
			if actualEndpoint != tt.wantEndpoint {
				t.Errorf("GetRegionalParameterManagerClient() with region '%s' has endpoint '%s', want '%s'", tt.region, actualEndpoint, tt.wantEndpoint)
			}
		})
	}
}
