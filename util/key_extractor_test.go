package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestExtractContentUsingJSONKey(t *testing.T) {
	tests := []struct {
		name          string
		payload       []byte
		key           string
		want          []byte
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:    "valid_json_key_exists_string_value",
			payload: []byte(`{"user": "admin", "role": "editor"}`),
			key:     "user",
			want:    []byte("admin"),
			wantErr: false,
		},
		{
			name:    "valid_json_key_exists_empty_string_value",
			payload: []byte(`{"token": ""}`),
			key:     "token",
			want:    []byte(""),
			wantErr: false,
		},
		{
			name:    "valid_json_key_exists_value_with_spaces",
			payload: []byte(`{"message": "hello world"}`),
			key:     "message",
			want:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "valid_json_key_exists_value_is_number",
			payload: []byte(`{"count": 123}`),
			key:     "count",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_json_key_exists_value_is_boolean",
			payload: []byte(`{"active": true}`),
			key:     "active",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_json_key_exists_value_is_null",
			payload: []byte(`{"nullable_field": null}`),
			key:     "nullable_field",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_json_key_exists_value_is_object",
			payload: []byte(`{"nested": {"a": "b"}}`),
			key:     "nested",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_json_key_exists_value_is_array",
			payload: []byte(`{"list": [1, 2, "item"]}`),
			key:     "list",
			want:    nil,
			wantErr: true,
		},
		{
			name:          "valid_json_key_does_not_exist",
			payload:       []byte(`{"user": "admin"}`),
			key:           "password",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "key 'password' not found in JSON",
		},
		{
			name:          "empty_json_object_key_does_not_exist",
			payload:       []byte(`{}`),
			key:           "anykey",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "key 'anykey' not found in JSON",
		},
		{
			name:          "invalid_json_unterminated_string",
			payload:       []byte(`{"user": "admin`),
			key:           "user",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "failed to unmarshal JSON",
		},
		{
			name:          "invalid_json_not_json_at_all",
			payload:       []byte(`this is not json`),
			key:           "user",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "failed to unmarshal JSON",
		},
		{
			name:          "empty_payload",
			payload:       []byte(``),
			key:           "user",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "failed to unmarshal JSON",
		},
		{
			name:    "valid_json_empty_key_exists",
			payload: []byte(`{"": "value_for_empty_key"}`),
			key:     "",
			want:    []byte("value_for_empty_key"),
			wantErr: false,
		},
		{
			name:          "valid_json_empty_key_does_not_exist",
			payload:       []byte(`{"user": "admin"}`),
			key:           "",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "key '' not found in JSON",
		},
		{
			name:    "valid_json_key_with_special_chars",
			payload: []byte(`{"key-with-hyphen!@#": "special_value"}`),
			key:     "key-with-hyphen!@#",
			want:    []byte("special_value"),
			wantErr: false,
		},
		{
			name:    "valid_json_value_with_special_chars",
			payload: []byte(`{"data": "!@#$%^&*()_+"}`),
			key:     "data",
			want:    []byte("!@#$%^&*()_+"),
			wantErr: false,
		},
		{
			name:    "valid_json_unicode_key_and_value",
			payload: []byte(`{"你好": "世界"}`),
			key:     "你好",
			want:    []byte("世界"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractContentUsingJSONKey(tt.payload, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractContentUsingJSONKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrSubstr != "" {
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("ExtractContentUsingJSONKey() error = %q, wantErrSubstr %q", err.Error(), tt.wantErrSubstr)
				}
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("ExtractContentUsingJSONKey() got = %s, want %s", string(got), string(tt.want))
			}
		})
	}
}

func TestExtractContentUsingYAMLKey(t *testing.T) {
	tests := []struct {
		name          string
		payload       []byte
		key           string
		want          []byte
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:    "valid_yaml_key_exists_string_value",
			payload: []byte("user: admin\nrole: editor"),
			key:     "user",
			want:    []byte("admin"),
			wantErr: false,
		},
		{
			name:    "valid_yaml_key_exists_empty_string_value",
			payload: []byte("token: \"\""),
			key:     "token",
			want:    []byte(""),
			wantErr: false,
		},
		{
			name:    "valid_yaml_key_exists_value_is_number_int",
			payload: []byte("count: 123"),
			key:     "count",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_number_float",
			payload: []byte("ratio: 1.23"),
			key:     "ratio",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_boolean_true",
			payload: []byte("active: true"),
			key:     "active",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_boolean_false",
			payload: []byte("enabled: false"),
			key:     "enabled",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_null_keyword",
			payload: []byte("nullable_field: null"),
			key:     "nullable_field",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_null_tilde",
			payload: []byte("another_null: ~"),
			key:     "another_null",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_object",
			payload: []byte("nested:\n  a: b\n  val: 10"),
			key:     "nested",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "valid_yaml_key_exists_value_is_array",
			payload: []byte("list:\n  - 1\n  - text\n  - true"),
			key:     "list",
			want:    nil,
			wantErr: true,
		},
		{
			name:          "valid_yaml_key_does_not_exist",
			payload:       []byte("user: admin"),
			key:           "password",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "key 'password' not found in YAML",
		},
		{
			name:          "empty_yaml_document_key_does_not_exist",
			payload:       []byte("---"), // Represents an empty document / nil map
			key:           "anykey",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "key 'anykey' not found in YAML",
		},
		{
			name:          "invalid_yaml_format",
			payload:       []byte("user: admin\n  badindent: true"),
			key:           "user",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "failed to unmarshal YAML",
		},
		{
			name:          "empty_payload_for_yaml",
			payload:       []byte(""),
			key:           "user",
			want:          nil,
			wantErr:       true,
			wantErrSubstr: "key 'user' not found in YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractContentUsingYAMLKey(tt.payload, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractContentUsingYAMLKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrSubstr != "" {
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("ExtractContentUsingYAMLKey() error = %q, wantErrSubstr %q", err.Error(), tt.wantErrSubstr)
				}
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("ExtractContentUsingYAMLKey() got = %s (%v), want %s (%v)", string(got), got, string(tt.want), tt.want)
			}
		})
	}
}
