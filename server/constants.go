// Copyright 2024 Google LLC
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

// Package server implements a grpc server to receive mount events
package server

const (
	locationLengthLimit = 30 // Maximum length of location is string is 30
	// #nosec G101 - Not actually hardcoded credentials
	regionalSecretRegex = "projects/([^/]+)/locations/([^/]+)/secrets/([^/]+)/versions/([^/]+)$"
	// #nosec G101 - Not actually hardcoded credentials
	globalSecretRegex = "projects/([^/]+)/secrets/([^/]+)/versions/([^/]+)$"
)