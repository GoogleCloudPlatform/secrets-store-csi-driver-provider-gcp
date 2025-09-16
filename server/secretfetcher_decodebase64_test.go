package server

import (
	"testing"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/util"
)

// TestBase64ProcessingLogic tests the base64 processing logic that would be used in secret fetching
func TestBase64ProcessingLogic(t *testing.T) {
	tests := []struct {
		name               string
		secretData         string
		extractJSONKey     string
		decodeBase64       bool
		expectedPayload    []byte
		expectError        bool
		expectedErrSubstr  string
	}{
		{
			name:            "decode base64 simple text",
			secretData:      "aGVsbG8gd29ybGQ=", // base64 for "hello world"
			decodeBase64:    true,
			expectedPayload: []byte("hello world"),
			expectError:     false,
		},
		{
			name:            "decode base64 json",
			secretData:      "eyJ1c2VyIjoiYWRtaW4iLCJwYXNzd29yZCI6InNlY3JldCJ9", // base64 for {"user":"admin","password":"secret"}
			decodeBase64:    true,
			expectedPayload: []byte("{\"user\":\"admin\",\"password\":\"secret\"}"),
			expectError:     false,
		},
		{
			name:            "no base64 decoding",
			secretData:      "plain text secret",
			decodeBase64:    false,
			expectedPayload: []byte("plain text secret"),
			expectError:     false,
		},
		{
			name:            "extract json key and decode base64",
			secretData:      "{\"data\":\"aGVsbG8gd29ybGQ=\"}", // JSON with base64 encoded value
			extractJSONKey:  "data",
			decodeBase64:    true,
			expectedPayload: []byte("hello world"),
			expectError:     false,
		},
		{
			name:              "invalid base64",
			secretData:        "invalid base64!!!",
			decodeBase64:      true,
			expectError:       true,
			expectedErrSubstr: "failed to decode base64 content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the processing logic that happens in the fetcher
			var content []byte
			var err error

			if tt.extractJSONKey != "" {
				content, err = util.ExtractContentUsingJSONKey([]byte(tt.secretData), tt.extractJSONKey)
				if err != nil {
					if tt.expectError && tt.expectedErrSubstr != "" {
						if !containsSubstring(err.Error(), tt.expectedErrSubstr) {
							t.Errorf("expected error to contain %q, got %q", tt.expectedErrSubstr, err.Error())
						}
					} else if !tt.expectError {
						t.Errorf("unexpected error during JSON key extraction: %v", err)
					}
					return
				}
			} else {
				content = []byte(tt.secretData)
			}

			// Apply base64 decoding if requested
			if tt.decodeBase64 {
				decoded, err := util.DecodeBase64Content(content)
				if err != nil {
					if tt.expectError && tt.expectedErrSubstr != "" {
						if !containsSubstring(err.Error(), tt.expectedErrSubstr) {
							t.Errorf("expected error to contain %q, got %q", tt.expectedErrSubstr, err.Error())
						}
					} else if !tt.expectError {
						t.Errorf("unexpected error during base64 decoding: %v", err)
					}
					return
				}
				content = decoded
			}

			if tt.expectError {
				t.Errorf("expected error but got none")
			} else {
				if string(content) != string(tt.expectedPayload) {
					t.Errorf("expected payload %q, got %q", string(tt.expectedPayload), string(content))
				}
			}
		})
	}
}

// containsSubstring checks if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}