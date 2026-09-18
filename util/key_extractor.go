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
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func ExtractContentUsingJSONKey(payload []byte, key string) ([]byte, error) {
	var data map[string]any
	err := json.Unmarshal(payload, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v. Invalid JSON format for key extraction", err)
	}
	value, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found in JSON", key)
	}
	return getValue(key, value)
}

func ExtractContentUsingYAMLKey(payload []byte, key string) ([]byte, error) {
	var data map[string]any
	err := yaml.Unmarshal(payload, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v. Invalid YAML format for key extraction", err)
	}
	value, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found in YAML", key)
	}
	return getValue(key, value)
}

func getValue(key string, value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported value type for key '%s'", key)
	}
}
