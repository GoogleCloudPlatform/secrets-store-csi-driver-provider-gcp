// Copyright 2020 Google LLC
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
	"os"
)

type EnvVar struct {
	envVarName   string
	defaultValue string
	isRequired   bool
}

func (ev EnvVar) GetValue() (string, error) {
	osEnv := ev.envVarName
	osEnvValue, isPresent := os.LookupEnv(osEnv)

	if isPresent {
		return osEnvValue, nil
	}

	if ev.isRequired {
		return "", fmt.Errorf("%s: a required OS environment is not present", osEnv)
	}

	// Return default endpoint
	return ev.defaultValue, nil
}

var IdentityBindingTokenEndPoint = EnvVar{
	envVarName:   "GAIA_TOKEN_EXCHANGE_ENDPOINT",
	defaultValue: "https://securetoken.googleapis.com/v1/identitybindingtoken",
	isRequired:   false,
}

var GkeWorkloadIdentityEndPoint = EnvVar{
	envVarName:   "GKE_WORKLOAD_IDENTITY_ENDPOINT",
	defaultValue: "https://container.googleapis.com/v1",
	isRequired:   false,
}
