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
	"os"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/compute/metadata"
	"k8s.io/klog/v2"
)

// DefaultUniverseDomain is the default Google Cloud universe domain (GDU).
const DefaultUniverseDomain = "googleapis.com"

var (
	universeDomainMu sync.RWMutex
	universeDomain   = DefaultUniverseDomain
)

// SetUniverseDomain sets the provider-wide resolved universe domain.
func SetUniverseDomain(ud string) {
	universeDomainMu.Lock()
	defer universeDomainMu.Unlock()
	trimmed := strings.TrimSpace(ud)
	if trimmed == "" {
		universeDomain = DefaultUniverseDomain
		return
	}
	universeDomain = trimmed
}

// GetUniverseDomain returns the provider-wide resolved universe domain.
func GetUniverseDomain() string {
	universeDomainMu.RLock()
	defer universeDomainMu.RUnlock()
	if universeDomain == "" {
		return DefaultUniverseDomain
	}
	return universeDomain
}

// IsDefaultUniverse returns true if the provider is running in the default
// Google Cloud universe ("googleapis.com").
func IsDefaultUniverse() bool {
	return GetUniverseDomain() == DefaultUniverseDomain
}

// ResolveUniverseDomain determines the Google Cloud universe domain using:
// 1. UNIVERSE_DOMAIN environment variable (if set and non-empty),
// 2. GCE metadata server "universe/universe-domain" attribute,
// 3. Fallback to DefaultUniverseDomain ("googleapis.com").
func ResolveUniverseDomain(ctx context.Context, metadataClient *metadata.Client) string {
	if ud, err := UniverseDomainEnv.GetValue(); err == nil && strings.TrimSpace(ud) != "" {
		return strings.TrimSpace(ud)
	}
	if metadataClient != nil {
		ud, err := metadataClient.GetWithContext(ctx, "universe/universe-domain")
		if err == nil && strings.TrimSpace(ud) != "" {
			return strings.TrimSpace(ud)
		}
		klog.V(4).InfoS("metadata server universe-domain lookup failed or empty, falling back to default universe", "err", err)
	}
	return DefaultUniverseDomain
}

// FormatGRPCEndpoint formats the global gRPC endpoint for a service in the active universe.
func FormatGRPCEndpoint(service string) string {
	return fmt.Sprintf("dns:///%s.%s:443", service, GetUniverseDomain())
}

type EnvVar struct {
	envVarName   string
	defaultValue string
	isRequired   bool
}

// HasProxyConfigured returns true if proxy configured for provider.
func HasProxyConfigured() bool {
	for _, v := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if _, ok := os.LookupEnv(v); ok {
			return true
		}
	}
	return false
}

func (ev EnvVar) GetValue() (string, error) {
	osEnv := ev.envVarName
	osEnvValue, isPresent := os.LookupEnv(osEnv)

	if isPresent {
		return resolveEndpointForUniverse(ev.envVarName, osEnvValue), nil
	}

	if ev.isRequired {
		return "", fmt.Errorf("%s: a required OS environment is not present", osEnv)
	}

	// Return default endpoint (adapted for active universe domain if applicable)
	return resolveEndpointForUniverse(ev.envVarName, ev.defaultValue), nil
}

func resolveEndpointForUniverse(envVarName, val string) string {
	if IsDefaultUniverse() {
		return val
	}
	ud := GetUniverseDomain()
	switch envVarName {
	case "GAIA_TOKEN_EXCHANGE_ENDPOINT":
		// If GAIA_TOKEN_EXCHANGE_ENDPOINT is unset or still pointing to the GDU
		// default securetoken endpoint in a non-default (TPC) universe, route token
		// exchange to the universe's STS endpoint.
		if val == "" || val == IdentityBindingTokenEndPoint.defaultValue {
			return fmt.Sprintf("https://sts.%s/v1/token", ud)
		}
	case "GKE_WORKLOAD_IDENTITY_ENDPOINT":
		if val == "" || val == GkeWorkloadIdentityEndPoint.defaultValue {
			return fmt.Sprintf("https://container.%s/v1", ud)
		}
	}
	return val
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

var UniverseDomainEnv = EnvVar{
	envVarName:   "UNIVERSE_DOMAIN",
	defaultValue: "",
	isRequired:   false,
}
