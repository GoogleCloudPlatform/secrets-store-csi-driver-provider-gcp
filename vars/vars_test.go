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

// Package vars includes storing and obtaining provider wide variables and their values
package vars

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/compute/metadata"
)

func setTestEnvVars(t *testing.T, envVars map[string]string) {
	for k, v := range envVars {
		t.Setenv(k, v)
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name    string
		in      EnvVar
		want    string
		wantErr error
		envVars map[string]string
	}{
		{
			name:    "default value",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "default_value", isRequired: false},
			want:    "default_value",
			wantErr: nil,
		},
		{
			name:    "env var present",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "default_value", isRequired: false},
			want:    "env_var_value",
			wantErr: nil,
			envVars: map[string]string{"TEST_ENV_VAR": "env_var_value"},
		},
		{
			name:    "env var not present but it is required",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "default_value", isRequired: true},
			wantErr: fmt.Errorf("TEST_ENV_VAR: a required OS environment is not present"),
			envVars: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setTestEnvVars(t, tc.envVars)
			got, err := tc.in.GetValue()
			if tc.wantErr != nil {
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Fatalf("GetValue(%v) returned an unexpected error: %v, want: %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if got != tc.want {
				t.Errorf("GetValue(%v) = %v, want: %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestGetBooleanValue(t *testing.T) {
	tests := []struct {
		name    string
		in      EnvVar
		want    bool
		wantErr error
		envVars map[string]string
	}{
		{
			name:    "default value when env var is not present",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "true", isRequired: false},
			want:    true,
			wantErr: nil,
		},
		{
			name:    "error when env value is misconigured",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "false", isRequired: false},
			want:    false,
			wantErr: fmt.Errorf("error parsing the boolean value: strconv.ParseBool: parsing \"not_a_bool\": invalid syntax"),
			envVars: map[string]string{"TEST_ENV_VAR": "not_a_bool"},
		},
		{
			name:    "env var present",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "false", isRequired: false},
			want:    true,
			wantErr: nil,
			envVars: map[string]string{"TEST_ENV_VAR": "true"},
		},
		{
			name:    "env var not present but it is required",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "false", isRequired: true},
			wantErr: fmt.Errorf("TEST_ENV_VAR: a required OS environment is not present"),
			envVars: map[string]string{},
		},
		{
			name:    "default value is misconfigured",
			in:      EnvVar{envVarName: "TEST_ENV_VAR", defaultValue: "not_a_bool", isRequired: false},
			wantErr: fmt.Errorf("error parsing default value: strconv.ParseBool: parsing \"not_a_bool\": invalid syntax"),
			envVars: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setTestEnvVars(t, tc.envVars)
			got, err := tc.in.GetBooleanValue()
			if tc.wantErr != nil {
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Fatalf("GetBooleanValue(%v) returned an unexpected error: %v, want: %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if got != tc.want {
				t.Errorf("GetBooleanValue(%v) = %v, want: %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestHasProxyConfigured(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    bool
	}{
		{
			name: "no proxy env vars",
			envVars: map[string]string{
				"UNRELATED_VAR": "value",
			},
			want: false,
		},
		{
			name:    "HTTPS_PROXY set",
			envVars: map[string]string{"HTTPS_PROXY": "proxy.example.com"},
			want:    true,
		},
		{
			name:    "https_proxy set",
			envVars: map[string]string{"https_proxy": "proxy.example.com"},
			want:    true,
		},
		{
			name:    "HTTP_PROXY set",
			envVars: map[string]string{"HTTP_PROXY": "proxy.example.com"},
			want:    true,
		},
		{
			name:    "http_proxy set",
			envVars: map[string]string{"http_proxy": "proxy.example.com"},
			want:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, proxyVar := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
				t.Setenv(proxyVar, "")
				os.Unsetenv(proxyVar)
			}
			setTestEnvVars(t, tc.envVars)
			got := HasProxyConfigured()
			if got != tc.want {
				t.Errorf("HasProxyConfigured() = %v, want: %v", got, tc.want)
			}
		})
	}
}

func TestUniverseDomainHelpers(t *testing.T) {
	t.Cleanup(func() {
		SetUniverseDomain(DefaultUniverseDomain)
	})

	SetUniverseDomain("")
	if got := GetUniverseDomain(); got != DefaultUniverseDomain {
		t.Errorf("GetUniverseDomain() = %q, want %q", got, DefaultUniverseDomain)
	}
	if !IsDefaultUniverse() {
		t.Errorf("IsDefaultUniverse() = false, want true")
	}

	SetUniverseDomain("apis-tpczero.goog")
	if got := GetUniverseDomain(); got != "apis-tpczero.goog" {
		t.Errorf("GetUniverseDomain() = %q, want %q", got, "apis-tpczero.goog")
	}
	if IsDefaultUniverse() {
		t.Errorf("IsDefaultUniverse() = true, want false")
	}
	if got := FormatGRPCEndpoint("secretmanager"); got != "dns:///secretmanager.apis-tpczero.goog:443" {
		t.Errorf("FormatGRPCEndpoint(secretmanager) = %q, want %q", got, "dns:///secretmanager.apis-tpczero.goog:443")
	}
}

func TestResolveUniverseDomain(t *testing.T) {
	t.Cleanup(func() {
		SetUniverseDomain(DefaultUniverseDomain)
	})

	tests := []struct {
		name           string
		envVal         string
		metadataVal    string
		metadataStatus int
		want           string
	}{
		{
			name:        "env var overrides metadata",
			envVal:      "env-universe.goog",
			metadataVal: "metadata-universe.goog",
			want:        "env-universe.goog",
		},
		{
			name:        "metadata fallback when env var empty",
			envVal:      "",
			metadataVal: "apis-tpczero.goog\n",
			want:        "apis-tpczero.goog",
		},
		{
			name:           "default universe fallback when metadata fails",
			envVal:         "",
			metadataStatus: http.StatusNotFound,
			want:           DefaultUniverseDomain,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("UNIVERSE_DOMAIN", tc.envVal)
			if tc.envVal == "" {
				os.Unsetenv("UNIVERSE_DOMAIN")
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Metadata-Flavor", "Google")
				if tc.metadataStatus != 0 && tc.metadataStatus != http.StatusOK {
					w.WriteHeader(tc.metadataStatus)
					return
				}
				fmt.Fprint(w, tc.metadataVal)
			}))
			defer srv.Close()

			t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(srv.URL, "http://"))
			mc := metadata.NewClient(srv.Client())

			got := ResolveUniverseDomain(context.Background(), mc)
			if got != tc.want {
				t.Errorf("ResolveUniverseDomain() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEndpointResolutionByUniverse(t *testing.T) {
	t.Cleanup(func() {
		SetUniverseDomain(DefaultUniverseDomain)
	})

	tests := []struct {
		name           string
		universeDomain string
		envVars        map[string]string
		wantTokenEP    string
		wantGkeWIEP    string
	}{
		{
			name:           "GDU defaults",
			universeDomain: DefaultUniverseDomain,
			wantTokenEP:    "https://securetoken.googleapis.com/v1/identitybindingtoken",
			wantGkeWIEP:    "https://container.googleapis.com/v1",
		},
		{
			name:           "TPC universe with unset env vars uses STS and TPC container endpoint",
			universeDomain: "apis-tpczero.goog",
			wantTokenEP:    "https://sts.apis-tpczero.goog/v1/token",
			wantGkeWIEP:    "https://container.apis-tpczero.goog/v1",
		},
		{
			name:           "TPC universe with legacy GDU GAIA_TOKEN_EXCHANGE_ENDPOINT overrides to STS",
			universeDomain: "apis-tpczero.goog",
			envVars: map[string]string{
				"GAIA_TOKEN_EXCHANGE_ENDPOINT": "https://securetoken.googleapis.com/v1/identitybindingtoken",
			},
			wantTokenEP: "https://sts.apis-tpczero.goog/v1/token",
			wantGkeWIEP: "https://container.apis-tpczero.goog/v1",
		},
		{
			name:           "TPC universe with explicit custom GAIA_TOKEN_EXCHANGE_ENDPOINT preserves override",
			universeDomain: "apis-tpczero.goog",
			envVars: map[string]string{
				"GAIA_TOKEN_EXCHANGE_ENDPOINT": "https://custom-sts.apis-tpczero.goog/v1/token",
			},
			wantTokenEP: "https://custom-sts.apis-tpczero.goog/v1/token",
			wantGkeWIEP: "https://container.apis-tpczero.goog/v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GAIA_TOKEN_EXCHANGE_ENDPOINT", "")
			os.Unsetenv("GAIA_TOKEN_EXCHANGE_ENDPOINT")
			t.Setenv("GKE_WORKLOAD_IDENTITY_ENDPOINT", "")
			os.Unsetenv("GKE_WORKLOAD_IDENTITY_ENDPOINT")
			setTestEnvVars(t, tc.envVars)
			SetUniverseDomain(tc.universeDomain)

			gotTokenEP, err := IdentityBindingTokenEndPoint.GetValue()
			if err != nil || gotTokenEP != tc.wantTokenEP {
				t.Errorf("IdentityBindingTokenEndPoint.GetValue() = %q, %v; want %q, nil", gotTokenEP, err, tc.wantTokenEP)
			}

			gotGkeWIEP, err := GkeWorkloadIdentityEndPoint.GetValue()
			if err != nil || gotGkeWIEP != tc.wantGkeWIEP {
				t.Errorf("GkeWorkloadIdentityEndPoint.GetValue() = %q, %v; want %q, nil", gotGkeWIEP, err, tc.wantGkeWIEP)
			}
		})
	}
}
