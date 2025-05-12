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
	"fmt"
	"strings"
	"testing"
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
