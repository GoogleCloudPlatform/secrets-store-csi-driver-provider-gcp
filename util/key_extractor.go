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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

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
	switch v := value.(type) {
	case string:
		return []byte(value.(string)), nil
	case map[string]any:
		return json.Marshal(v)
	case []any:
		return json.Marshal(v)
	case int:
		return anyToBytesConvertInt(int64(v))
	case float64:
		return anyToBytesFloat64(v)
	case bool:
		return anyToBytesBool(value.(bool))
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported value type for key '%s'", key)
	}
}

func ExtractContentUsingYAMLKey(payload []byte, key string) ([]byte, error) {
	var data map[any]any
	err := yaml.Unmarshal(payload, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v. Invalid YAML format for key extraction", err)
	}
	value, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found in YAML", key)
	}
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case map[string]any:
		return yaml.Marshal(v)
	case map[any]any:
		return yaml.Marshal(v)
	case []any:
		return yaml.Marshal(v)
	case int:
		return anyToBytesConvertInt(int64(v))
	case float64:
		return anyToBytesFloat64(v)
	case bool:
		return anyToBytesBool(v)
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported value type for key '%s'", key)
	}
}

func anyToBytesConvertInt(val int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, val)
	return buf.Bytes(), err
}

func anyToBytesBool(val bool) ([]byte, error) {
	if val {
		return []byte{1}, nil
	}
	return []byte{0}, nil
}

func anyToBytesFloat64(val float64) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, math.Float64bits(val))
	if err != nil {
		return nil, fmt.Errorf("failed to encode float64: %w", err)
	}
	return buf.Bytes(), nil
}
