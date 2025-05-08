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

// Package util provides utility methods to be used across other packages
package util

import (
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExtractLocationFromSecretResource returns location from the secret resource if the resource is in format "projects/<project_id>/locations/<location_id>/..."
// returns "" for global secret resource.
func ExtractLocationFromSecretResource(resource string) (string, error) {
	globalSecretRegexp := regexp.MustCompile(globalSecretRegex)
	if m := globalSecretRegexp.FindStringSubmatch(resource); m != nil {
		return "", nil
	}
	regionalSecretRegexp := regexp.MustCompile(regionalSecretRegex)
	if m := regionalSecretRegexp.FindStringSubmatch(resource); m != nil {
		if len(m[2]) > locationLengthLimit {
			return "", status.Errorf(codes.InvalidArgument, "Invalid location: %s, location length exceeds limit", m[2])
		}
		return m[2], nil
	}
	return "", status.Errorf(codes.InvalidArgument, "Invalid secret resource name: %s", resource)
}

// ExtractLocationFromParameterManagerResource returns location from the parameter resource
// if the resource is in the following format for global resource
// "projects/<project_id>/locations/global/parameters/<parameter_name>/versions/<pm_version_name>"
// or in the following format for regionalized resource
// "projects/<project_id>/locations/<location_id>/parameters/<parameter_name>/versions/<pm_version_name>"
// returns "" for global parameter resource.
func ExtractLocationFromParameterManagerResource(resource string) (string, error) {
	globalParameterRegexp := regexp.MustCompile(globalParameterVersionRegex)
	if m := globalParameterRegexp.FindStringSubmatch(resource); m != nil {
		return "", nil
	}
	regionalParameterRegexp := regexp.MustCompile(regionalParameterVersionRegex)
	if m := regionalParameterRegexp.FindStringSubmatch(resource); m != nil {
		if len(m[2]) > locationLengthLimit {
			return "", status.Errorf(codes.InvalidArgument, "Invalid location: %s", m[2])
		}
		return m[2], nil
	}
	return "", status.Errorf(codes.InvalidArgument, "Invalid parameter resource name: %s", resource)
}
