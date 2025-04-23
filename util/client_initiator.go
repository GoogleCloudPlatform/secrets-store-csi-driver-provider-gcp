// Copyright 2020 Google LLC
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
	"google.golang.org/api/option"
	"k8s.io/klog/v2"
)

var pmRegions = []string{
	"us-central1", "us-east4", "europe-west1", "europe-west4", "europe-west2",
	"us-east7", "europe-west3",
}

// sm probably has more regions they will be initialised in server.go as per the regions required
var smRegions = []string{
	"us-central1", "us-east4", "europe-west1", "europe-west4", "europe-west2",
	"us-east7", "europe-west3",
}

func GetRegionalSecretManagerClient(region string, clientOptions []option.ClientOption) *secretmanager.Client {
	// See https://pkg.go.dev/cloud.google.com/go#hdr-Client_Options
	regionalClient, err := secretmanager.NewClient(context.Background(),
		append(clientOptions, option.WithEndpoint(fmt.Sprintf("secretmanager.%s.googleapis.com:443", region)))...)
	if err != nil {
		klog.ErrorS(err, "failed to create secret manager client for region", region)
		return nil
	}
	return regionalClient
}

func GetRegionalParameterManagerClient(region string, clientOptions []option.ClientOption) *parametermanager.Client {
	// See https://pkg.go.dev/cloud.google.com/go#hdr-Client_Options
	regionalClient, err := parametermanager.NewClient(context.Background(),
		append(clientOptions, option.WithEndpoint(fmt.Sprintf("parametermanager.%s.rep.googleapis.com:443", region)))...)
	if err != nil {
		klog.ErrorS(err, "failed to create parameter manager client for region", region)
		return nil
	}
	return regionalClient
}

func InitializeSecretManagerRegionalMap(ctx context.Context, clientOptions []option.ClientOption) map[string]*secretmanager.Client {
	// To cache the clients for secret manager regional endpoints
	smRep := make(map[string]*secretmanager.Client)
	// Initialize the map with regional endpoints
	for _, region := range smRegions {
		// See https://pkg.go.dev/cloud.google.com/go#hdr-Client_Options
		regionalClient := GetRegionalSecretManagerClient(region, clientOptions)
		if regionalClient != nil {
			smRep[region] = regionalClient
		}
	}
	return smRep
}

func InitializeParameterManagerRegionalMap(ctx context.Context, clientOptions []option.ClientOption) map[string]*parametermanager.Client {
	// To cache the clients for parameter manager regional endpoints
	pmRep := make(map[string]*parametermanager.Client)
	// Initialize the map with regional endpoints
	for _, region := range pmRegions {
		// See https://pkg.go.dev/cloud.google.com/go#hdr-Client_Options
		regionalClient := GetRegionalParameterManagerClient(region, clientOptions)
		if regionalClient != nil {
			pmRep[region] = regionalClient
		}
	}
	return pmRep
}
