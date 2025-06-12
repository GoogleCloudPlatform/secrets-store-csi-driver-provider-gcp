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
	"os"
	"strconv"
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

func (ev EnvVar) GetBooleanValue() (bool, error) {
	oEnvValue, isPresent := os.LookupEnv(ev.envVarName)

	if isPresent {
		boolValue, err := strconv.ParseBool(oEnvValue)
		if err != nil {
			return false, fmt.Errorf("error parsing the boolean value: %v", err)
		}
		return boolValue, nil
	}

	if ev.isRequired {
		return false, fmt.Errorf("%s: a required OS environment is not present", ev.envVarName)
	}
	defaultBoolValue, err := strconv.ParseBool(ev.defaultValue)
	if err != nil {
		return false, fmt.Errorf("error parsing default value: %v", err)
	}
	return defaultBoolValue, nil
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

var ProviderName = EnvVar{
	envVarName:   "PROVIDER_NAME",
	defaultValue: "gcp",
	isRequired:   false,
}

var UserAgentIdentifier = EnvVar{
	envVarName:   "USER_AGENT",
	defaultValue: "secrets-store-csi-driver-provider-gcp",
	isRequired:   false,
}

var AllowNodepublishSecretRef = EnvVar{
	envVarName:   "ALLOW_NODE_PUBLISH_SECRET",
	defaultValue: "false",
	isRequired:   false,
}

var Project = EnvVar{
	envVarName:   "PROJECT",
	defaultValue: "",
	isRequired:   false,
}

var ClusterName = EnvVar{
	envVarName:   "CLUSTER_NAME",
	defaultValue: "",
	isRequired:   false,
}

var ClusterLocation = EnvVar{
	envVarName:   "CLUSTER_LOCATION",
	defaultValue: "",
	isRequired:   false,
}
