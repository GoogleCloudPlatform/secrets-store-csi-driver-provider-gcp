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
	"regexp"
)

// IsSecretResource returns true/false depending on whether the resource URI satisfies the given
// globalSecretRegex/regionalizedSecretRegex
func IsSecretResource(resource string) bool {
	globalSecretRegexp := regexp.MustCompile(globalSecretRegex)
	regionalSecretRegexp := regexp.MustCompile(regionalSecretRegex)
	return globalSecretRegexp.MatchString(resource) || regionalSecretRegexp.MatchString(resource)
}

// IsParameterManagerResource returns true/false depending on whether the resource URI satisfies the given
// globalParameterVersionRegex/regionalParameterVersionRegex
func IsParameterManagerResource(resource string) bool {
	globalParameterVersionRegexp := regexp.MustCompile(globalParameterVersionRegex)
	regionalParameterVersionRegexp := regexp.MustCompile(regionalParameterVersionRegex)
	return globalParameterVersionRegexp.MatchString(resource) || regionalParameterVersionRegexp.MatchString(resource)
}
