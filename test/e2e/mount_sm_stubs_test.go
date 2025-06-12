// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !secretmanager_e2e && !all_e2e
// +build !secretmanager_e2e,!all_e2e

package test

// Stubs for Secret Manager setup/teardown when SM tests are not included.
func setupSmTestSuite() {
	// Do nothing. The actual implementation is in mount_secretmanager_test.go
}

func teardownSmTestSuite() {
	// Do nothing. The actual implementation is in mount_secretmanager_test.go
}
