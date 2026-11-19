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

// Package util provides utility methods to be used across other packages
package util

import (
	"context"
	"fmt"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/vars"
	"google.golang.org/api/option"
	"k8s.io/klog/v2"
)

var newSMRegionalClientFunc = secretmanager.NewClient

var newSMRegionalRESTClientFunc = secretmanager.NewRESTClient

var newPMRegionalClientFunc = parametermanager.NewClient

func GetRegionalSecretManagerClient(ctx context.Context, region string, clientOptions []option.ClientOption) *secretmanager.Client {
	if !vars.IsDefaultUniverse() {
		universeDomain := vars.GetUniverseDomain()
		endpoint := fmt.Sprintf("secretmanager.%s:443", universeDomain)
		allOpts := append(clientOptions, option.WithUniverseDomain(universeDomain), option.WithEndpoint(endpoint))
		regionalClient, err := newSMRegionalRESTClientFunc(ctx, allOpts...)
		if err != nil {
			klog.ErrorS(err, "failed to create regional REST secret manager client for region", "region", region, "universeDomain", universeDomain)
			return nil
		}
		return regionalClient
	}

	// See https://pkg.go.dev/cloud.google.com/go#hdr-Client_Options
	regionalEndpoint := fmt.Sprintf("secretmanager.%s.rep.googleapis.com:443", region)
	allOpts := append(clientOptions, option.WithEndpoint(regionalEndpoint))
	regionalClient, err := newSMRegionalClientFunc(ctx, allOpts...)

	if err != nil {
		klog.ErrorS(err, "failed to create secret manager client for region", region)
		return nil
	}
	return regionalClient
}

func GetRegionalParameterManagerClient(ctx context.Context, region string, clientOptions []option.ClientOption) *parametermanager.Client {
	// See https://pkg.go.dev/cloud.google.com/go#hdr-Client_Options
	regionalEndpoint := fmt.Sprintf("parametermanager.%s.rep.googleapis.com:443", region)
	allOpts := append(clientOptions, option.WithEndpoint(regionalEndpoint))
	regionalClient, err := newPMRegionalClientFunc(ctx, allOpts...)
	if err != nil {
		klog.ErrorS(err, "failed to create parameter manager client for region", region)
		return nil
	}
	return regionalClient
}
