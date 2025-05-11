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
	"fmt"
	"testing"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"

	"google.golang.org/api/option"
)

func TestGetRegionalSecretManagerClient(t *testing.T) {
	endpointOptionTypeString := fmt.Sprintf("%T", option.WithEndpoint(""))
	ctx := context.Background()
	baseOpts := []option.ClientOption{option.WithoutAuthentication()}

	tests := []struct {
		name          string
		region        string
		clientOptions []option.ClientOption
		wantNil       bool
		wantEndpoint  string
		newClientErr  error
	}{
		{
			name:          "valid region",
			region:        "us-central1",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "secretmanager.us-central1.rep.googleapis.com:443",
			newClientErr:  nil,
		},
		{
			name:          "another valid region",
			region:        "europe-west1",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "secretmanager.europe-west1.rep.googleapis.com:443",
			newClientErr:  nil,
		},
		{
			name:          "new client returns error",
			region:        "us-east1",
			clientOptions: baseOpts,
			wantNil:       true,
			wantEndpoint:  "secretmanager.us-east1.rep.googleapis.com:443",
			newClientErr:  fmt.Errorf("simulated NewClient error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalNewSMClientFunc := newSMRegionalClientFunc
			defer func() { newSMRegionalClientFunc = originalNewSMClientFunc }()

			var capturedEndpoint string
			newSMRegionalClientFunc = func(ctx context.Context, opts ...option.ClientOption) (*secretmanager.Client, error) {
				if tt.newClientErr != nil {
					return nil, tt.newClientErr
				}
				for _, opt := range opts {
					if fmt.Sprintf("%T", opt) == endpointOptionTypeString {
						capturedEndpoint = fmt.Sprintf("%v", opt)
						break
					}
				}
				return secretmanager.NewClient(ctx, option.WithoutAuthentication(), option.WithEndpoint("localhost:1"))
			}

			client := GetRegionalSecretManagerClient(ctx, tt.region, tt.clientOptions)
			if tt.wantNil {
				if client != nil {
					t.Errorf("GetRegionalSecretManagerClient() with region '%s' = non-nil, want nil", tt.region)
					client.Close() // Attempt to close if unexpectedly non-nil
				}
				// If newClientErr was set and wantNil is true, this is the expected path.
				// No further checks on endpoint needed.
				return
			}

			// If wantNil is false, we expect a non-nil client.
			if client == nil {
				t.Fatalf("GetRegionalSecretManagerClient() with region '%s' = nil, want non-nil client. Mock NewClient error: %v", tt.region, tt.newClientErr)
			}

			// Client is not nil here, so deferring Close is safe.
			defer func() {
				if err := client.Close(); err != nil {
					t.Logf("Error closing client for region '%s': %v", tt.region, err)
				}
			}()

			if capturedEndpoint != tt.wantEndpoint {
				t.Errorf("GetRegionalSecretManagerClient() with region '%s' called NewClient with endpoint '%s', want '%s'", tt.region, capturedEndpoint, tt.wantEndpoint)
			}
		})
	}
}

func TestGetRegionalParameterManagerClient(t *testing.T) {
	// Determine the type string for an endpoint option.
	endpointOptionTypeString := fmt.Sprintf("%T", option.WithEndpoint(""))
	ctx := context.Background()
	baseOpts := []option.ClientOption{option.WithoutAuthentication()}

	tests := []struct {
		name          string
		region        string
		clientOptions []option.ClientOption
		wantNil       bool
		wantEndpoint  string
		newClientErr  error
	}{
		{
			name:          "valid region",
			region:        "europe-west3",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "parametermanager.europe-west3.rep.googleapis.com:443",
			newClientErr:  nil,
		},
		{
			name:          "another valid region",
			region:        "us-east7",
			clientOptions: baseOpts,
			wantNil:       false,
			wantEndpoint:  "parametermanager.us-east7.rep.googleapis.com:443",
			newClientErr:  nil,
		},
		{
			name:          "new client returns error",
			region:        "asia-south1",
			clientOptions: baseOpts,
			wantNil:       true,
			wantEndpoint:  "parametermanager.asia-south1.rep.googleapis.com:443",
			newClientErr:  fmt.Errorf("simulated NewClient error for parameter manager"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalNewPMClientFunc := newPMRegionalClientFunc
			defer func() { newPMRegionalClientFunc = originalNewPMClientFunc }()

			var capturedEndpoint string
			newPMRegionalClientFunc = func(ctx context.Context, opts ...option.ClientOption) (*parametermanager.Client, error) {
				if tt.newClientErr != nil {
					return nil, tt.newClientErr
				}
				for _, opt := range opts {
					if fmt.Sprintf("%T", opt) == endpointOptionTypeString {
						capturedEndpoint = fmt.Sprintf("%v", opt)
						break
					}
				}
				return parametermanager.NewClient(ctx, option.WithoutAuthentication(), option.WithEndpoint("localhost:1"))
			}

			client := GetRegionalParameterManagerClient(ctx, tt.region, tt.clientOptions)

			if tt.wantNil {
				if client != nil {
					t.Errorf("GetRegionalParameterManagerClient() with region '%s' = non-nil, want nil", tt.region)
					client.Close()
				}
				return
			}

			if client == nil {
				t.Fatalf("GetRegionalParameterManagerClient() with region '%s' = nil, want non-nil client. Mock NewClient error: %v", tt.region, tt.newClientErr)
			}

			// Client is not nil here, so deferring Close is safe.
			defer func() {
				if err := client.Close(); err != nil {
					t.Logf("Error closing client for region '%s': %v", tt.region, err)
				}
			}()

			if capturedEndpoint != tt.wantEndpoint {
				t.Errorf("GetRegionalParameterManagerClient() with region '%s' called NewClient with endpoint '%s', want '%s'", tt.region, capturedEndpoint, tt.wantEndpoint)
			}
		})
	}
}
