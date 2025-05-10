package util

import (
	"strings"
	"testing"
)

func TestExtractLocationFromSecretResource(t *testing.T) {
	tests := []struct {
		name          string
		resource      string
		wantLocation  string
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:         "valid_global_secret",
			resource:     "projects/my-project/secrets/my-secret/versions/latest",
			wantLocation: "",
			wantErr:      false,
		},
		{
			name:         "valid_regional_secret",
			resource:     "projects/my-project/locations/us-central1/secrets/my-secret/versions/1",
			wantLocation: "us-central1",
			wantErr:      false,
		},
		{
			name:          "invalid_location_too_long",
			resource:      "projects/my-project/locations/averylonglocationnamethatexceedsthelimitallowed/secrets/my-secret/versions/1",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid location",
		},
		{
			name:          "invalid_secret_format_missing_versions",
			resource:      "projects/my-project/secrets/my-secret",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid secret resource name",
		},
		{
			name:          "invalid_secret_format_regional_missing_versions",
			resource:      "projects/my-project/locations/us-east1/secrets/my-secret",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid secret resource name",
		},
		{
			name:          "empty_resource_string",
			resource:      "",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid secret resource name",
		},
		{
			name:          "parameter_manager_resource_passed_as_secret",
			resource:      "projects/my-project/locations/global/parameters/my-param/versions/1",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid secret resource name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLocation, err := ExtractLocationFromSecretResource(tt.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractLocationFromSecretResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrSubstr != "" {
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("ExtractLocationFromSecretResource() error = %q, wantErrSubstr %q", err.Error(), tt.wantErrSubstr)
				}
			}
			if gotLocation != tt.wantLocation {
				t.Errorf("ExtractLocationFromSecretResource() gotLocation = %v, want %v", gotLocation, tt.wantLocation)
			}
		})
	}
}

func TestExtractLocationFromParameterManagerResource(t *testing.T) {
	tests := []struct {
		name          string
		resource      string
		wantLocation  string
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:         "valid_global_parameter",
			resource:     "projects/my-project/locations/global/parameters/my-param/versions/latest",
			wantLocation: "",
			wantErr:      false,
		},
		{
			name:         "valid_regional_parameter",
			resource:     "projects/my-project/locations/europe-west1/parameters/my-param/versions/2",
			wantLocation: "europe-west1",
			wantErr:      false,
		},
		{
			name:          "invalid_location_too_long",
			resource:      "projects/my-project/locations/anotherverylonglocationnamethatexceedsthelimitallowed/parameters/my-param/versions/1",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid location",
		},
		{
			name:          "invalid_parameter_format_missing_versions",
			resource:      "projects/my-project/locations/global/parameters/my-param",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid parameter resource name",
		},
		{
			name:          "invalid_parameter_format_regional_missing_versions",
			resource:      "projects/my-project/locations/us-west1/parameters/my-param",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid parameter resource name",
		},
		{
			name:          "empty_resource_string",
			resource:      "",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid parameter resource name",
		},
		{
			name:          "secret_manager_resource_passed_as_parameter",
			resource:      "projects/my-project/secrets/my-secret/versions/latest",
			wantLocation:  "",
			wantErr:       true,
			wantErrSubstr: "Invalid parameter resource name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLocation, err := ExtractLocationFromParameterManagerResource(tt.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractLocationFromParameterManagerResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrSubstr != "" {
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("ExtractLocationFromParameterManagerResource() error = %q, wantErrSubstr %q", err.Error(), tt.wantErrSubstr)
				}
			}
			if gotLocation != tt.wantLocation {
				t.Errorf("ExtractLocationFromParameterManagerResource() gotLocation = %v, want %v", gotLocation, tt.wantLocation)
			}
		})
	}
}
