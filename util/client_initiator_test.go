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
package util

import (
	"context"
	"sort" // Import the standard sort package
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/option"
)

// Helper function to get keys from a map
func getMapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestInitializeSecretManagerRegionalMap(t *testing.T) {
	opts := []option.ClientOption{option.WithoutAuthentication()}
	ctx := context.Background()

	smMap := InitializeSecretManagerRegionalMap(ctx, opts)

	if len(smMap) != len(sm_regions) {
		t.Errorf("Expected map size %d, got %d", len(sm_regions), len(smMap))
	}

	// Check if all expected regions are present as keys
	// Make copies to avoid modifying the original sm_regions slice
	expectedKeys := make([]string, len(sm_regions))
	copy(expectedKeys, sm_regions)
	actualKeys := getMapKeys(smMap)

	// Sort slices for consistent comparison using sort.Strings
	sort.Strings(expectedKeys)
	sort.Strings(actualKeys)

	if diff := cmp.Diff(expectedKeys, actualKeys); diff != "" {
		t.Errorf("Map keys mismatch (-want +got):\n%s", diff)
	}

	for region, client := range smMap {
		if client == nil {
			t.Errorf("Client for region %s is nil", region)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("Warning: failed to close client for region %s: %v", region, err)
		}
	}
}

func TestInitializeParameterManagerRegionalMap(t *testing.T) {
	opts := []option.ClientOption{option.WithoutAuthentication()}
	ctx := context.Background()

	pmMap := InitializeParameterManagerRegionalMap(ctx, opts)

	// Check map size
	if len(pmMap) != len(pm_regions) {
		t.Errorf("Expected map size %d, got %d", len(pm_regions), len(pmMap))
	}

	// Check keys
	// Make copies to avoid modifying the original pm_regions slice
	expectedKeys := make([]string, len(pm_regions))
	copy(expectedKeys, pm_regions)
	actualKeys := getMapKeys(pmMap)

	// Sort slices for consistent comparison using sort.Strings
	sort.Strings(expectedKeys)
	sort.Strings(actualKeys)

	if diff := cmp.Diff(expectedKeys, actualKeys); diff != "" {
		t.Errorf("Map keys mismatch (-want +got):\n%s", diff)
	}

	for region, client := range pmMap {
		if client == nil {
			t.Errorf("Client for region %s is nil", region)
		}
		if err := client.Close(); err != nil {
			t.Logf("Warning: failed to close client for region %s: %v", region, err)
		}
	}
}
