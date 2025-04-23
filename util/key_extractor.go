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
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32:
		return anyToBytesConvertInt(value.(int64))
	case bool:
		return anyToBytesBool(value.(bool))
	case float32, float64:
		return anyToBytesFloat64(value.(float64))
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
		return nil, fmt.Errorf("failed to unmarshal JSON: %v. Invalid JSON format for key extraction", err)
	}
	value, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found in JSON", key)
	}
	switch v := value.(type) {
	case string:
		return []byte(value.(string)), nil
	case map[any]any:
		return yaml.Marshal(v)
	case []any:
		return yaml.Marshal(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32:
		return anyToBytesConvertInt(value.(int64))
	case bool:
		return anyToBytesBool(value.(bool))
	case float64:
		return anyToBytesFloat64(value.(float64))
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported value type for key '%s'", key)
	}
}

func anyToBytesConvertInt(val int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, val)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
