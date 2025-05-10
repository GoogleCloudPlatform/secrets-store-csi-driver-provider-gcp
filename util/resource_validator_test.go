package util

import (
	"testing"
)

func TestIsSecretResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		want     bool
	}{
		{
			name:     "valid global secret latest version",
			resource: "projects/my-project/secrets/my-secret/versions/latest",
			want:     true,
		},
		{
			name:     "valid global secret specific version",
			resource: "projects/my-project/secrets/my-secret/versions/123",
			want:     true,
		},
		{
			name:     "valid regional secret latest version",
			resource: "projects/my-project/locations/us-central1/secrets/my-secret/versions/latest",
			want:     true,
		},
		{
			name:     "valid regional secret specific version",
			resource: "projects/my-project/locations/europe-west1/secrets/my-secret/versions/5",
			want:     true,
		},
		{
			name:     "invalid - missing version",
			resource: "projects/my-project/secrets/my-secret",
			want:     false,
		},
		{
			name:     "invalid - missing version regional",
			resource: "projects/my-project/locations/us-central1/secrets/my-secret",
			want:     false,
		},
		{
			name:     "invalid - parameter manager resource global",
			resource: "projects/my-project/locations/global/parameters/my-param/versions/1",
			want:     false,
		},
		{
			name:     "invalid - parameter manager resource regional",
			resource: "projects/my-project/locations/us-east1/parameters/my-param/versions/latest",
			want:     false,
		},
		{
			name:     "invalid - wrong format",
			resource: "secrets/my-secret/versions/latest",
			want:     false,
		},
		{
			name:     "invalid - empty string",
			resource: "",
			want:     false,
		},
		{
			name:     "invalid - random string",
			resource: "this-is-not-a-resource-string",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSecretResource(tt.resource); got != tt.want {
				t.Errorf("IsSecretResource(%q) = %v, want %v", tt.resource, got, tt.want)
			}
		})
	}
}

func TestIsParameterManagerResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		want     bool
	}{
		{
			name:     "valid global parameter latest version",
			resource: "projects/my-project/locations/global/parameters/my-param/versions/latest",
			want:     true,
		},
		{
			name:     "valid global parameter specific version",
			resource: "projects/my-project/locations/global/parameters/my-param/versions/1",
			want:     true,
		},
		{
			name:     "valid regional parameter latest version",
			resource: "projects/my-project/locations/us-central1/parameters/my-param/versions/latest",
			want:     true,
		},
		{
			name:     "valid regional parameter specific version",
			resource: "projects/my-project/locations/europe-west1/parameters/my-param/versions/5",
			want:     true,
		},
		{
			name:     "invalid - missing version global",
			resource: "projects/my-project/locations/global/parameters/my-param",
			want:     false,
		},
		{
			name:     "invalid - missing version regional",
			resource: "projects/my-project/locations/us-central1/parameters/my-param",
			want:     false,
		},
		{
			name:     "invalid - secret manager resource global",
			resource: "projects/my-project/secrets/my-secret/versions/latest",
			want:     false,
		},
		{
			name:     "invalid - secret manager resource regional",
			resource: "projects/my-project/locations/us-east1/secrets/my-secret/versions/1",
			want:     false,
		},
		{
			name:     "invalid - wrong format",
			resource: "parameters/my-param/versions/latest",
			want:     false,
		},
		{
			name:     "invalid - empty string",
			resource: "",
			want:     false,
		},
		{
			name:     "invalid - random string",
			resource: "this-is-not-a-resource-string",
			want:     false,
		},
		{
			name:     "invalid - global location for secret (looks like param)",
			resource: "projects/my-project/locations/global/secrets/my-secret/versions/1",
			want:     false, // Secrets don't use 'global' location identifier
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsParameterManagerResource(tt.resource); got != tt.want {
				t.Errorf("IsParameterManagerResource(%q) = %v, want %v", tt.resource, got, tt.want)
			}
		})
	}
}
