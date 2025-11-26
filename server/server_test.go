// Copyright 2025 Google LLC
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

package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	"cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

const regionalParameterVersion = "projects/project/locations/us-central1/parameters/parameterIdRegional/versions/versionId"
const globalParameterVersion = "projects/project/locations/global/parameters/parameterIdGlobal/versions/versionId"
const globalParameterVersion2 = "projects/project/locations/global/parameters/parameterIdGlobal/versions/versionId2"

func TestHandleMountEvent(t *testing.T) {
	parameterManagerFileMode := int32(0500) // decimal 320
	secretFileMode := int32(0600)           // decimal 384

	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good2.txt",
				Mode:         &secretFileMode,
			},
			{
				ResourceName:   "projects/project/secrets/secretId/versions/latest",
				FileName:       "good3.txt",
				ExtractYAMLKey: "password",
			},
			{
				ResourceName:   globalParameterVersion,
				FileName:       "pm_good1.txt",
				ExtractJSONKey: "user",
			},
			{
				ResourceName: globalParameterVersion2,
				FileName:     "pm_good2.txt",
				Mode:         &parameterManagerFileMode,
			},
			{
				ResourceName:   regionalParameterVersion,
				FileName:       "pm_good3.txt",
				ExtractYAMLKey: "password",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	want := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{
			{
				Id:      "projects/project/secrets/test/versions/latest",
				Version: "projects/project/secrets/test/versions/2",
			},
			{
				Id:      "projects/project/secrets/test/versions/latest",
				Version: "projects/project/secrets/test/versions/2",
			},
			{
				Id:      "projects/project/secrets/secretId/versions/latest",
				Version: "projects/project/secrets/secretId/versions/1",
			},
			{
				Id:      globalParameterVersion,
				Version: globalParameterVersion,
			},
			{
				Id:      globalParameterVersion2,
				Version: globalParameterVersion2,
			},
			{
				Id:      regionalParameterVersion,
				Version: regionalParameterVersion,
			},
		},
		Files: []*v1alpha1.File{
			{
				Path:     "good1.txt",
				Mode:     777,
				Contents: []byte("My Secret"),
			},
			{
				Path:     "good2.txt",
				Mode:     384, // octal 0600 as 6*64 = 384
				Contents: []byte("My Secret"),
			},
			{
				Path:     "good3.txt",
				Mode:     777,
				Contents: []byte("password@1234"),
			},
			{
				Path:     "pm_good1.txt",
				Mode:     777, // octal 0500 as 5*64 = 320
				Contents: []byte("admin"),
			},
			{
				Path:     "pm_good2.txt",
				Mode:     320, // octal 0500 as 5*64 = 320
				Contents: []byte("user: admin\npassword: password@1234"),
			},
			{
				Path:     "pm_good3.txt",
				Mode:     777,
				Contents: []byte("password@1234"),
			},
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			if req.Name == "projects/project/secrets/secretId/versions/latest" {
				return &secretmanagerpb.AccessSecretVersionResponse{
					Name: "projects/project/secrets/secretId/versions/1",
					Payload: &secretmanagerpb.SecretPayload{
						Data: []byte("password: password@1234"),
					},
				}, nil
			}
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("My Secret"),
				},
			}, nil
		},
	})

	pmClient := mockParameterManagerClient(t, &mockParameterManagerServer{
		renderFn: func(ctx context.Context, req *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
			if req.Name == globalParameterVersion {
				data := []byte("{\"user\":\"admin\", \"password\":\"password@1234\"}")
				return &parametermanagerpb.RenderParameterVersionResponse{
					ParameterVersion: globalParameterVersion,
					RenderedPayload:  data,
				}, nil
			}
			data := []byte("user: admin\npassword: password@1234")
			return &parametermanagerpb.RenderParameterVersionResponse{
				ParameterVersion: globalParameterVersion2,
				RenderedPayload:  data,
			}, nil
		},
	})

	regionalPmClient := mockParameterManagerClient(t, &mockParameterManagerServer{
		renderFn: func(ctx context.Context, _ *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
			data := []byte("user: admin\npassword: password@1234")
			return &parametermanagerpb.RenderParameterVersionResponse{
				ParameterVersion: regionalParameterVersion,
				RenderedPayload:  data,
			}, nil
		},
	})

	regionalSmClients := make(map[string]*secretmanager.Client)
	regionalPmClients := make(map[string]*parametermanager.Client)
	regionalPmClients["us-central1"] = regionalPmClient

	server := &Server{
		SecretClient:                    client,
		ParameterManagerClient:          pmClient,
		RegionalSecretClients:           regionalSmClients,
		ServerClientOptions:             []option.ClientOption{},
		RegionalParameterManagerClients: regionalPmClients,
	}
	got, err := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}

func TestHandleMountBothSMKeyJSONYAMLKeyProvided(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName:   "projects/project/secrets/test/versions/latest",
				FileName:       "good1.txt",
				ExtractJSONKey: "user",
				ExtractYAMLKey: "password",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("user: admin\npassword: password@1234"),
				},
			}, nil
		},
	})
	regionalSmClients := make(map[string]*secretmanager.Client)

	server := &Server{
		SecretClient:          client,
		RegionalSecretClients: regionalSmClients,
		ServerClientOptions:   []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)

	if !strings.Contains(got.Error(), "Internal") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "both ExtractJSONKey and ExtractYAMLKey can't be simultaneously non empty strings") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountBothPMKeyJSONYAMLKeyProvided(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName:   globalParameterVersion,
				FileName:       "pm_good1.txt",
				ExtractJSONKey: "user",
				ExtractYAMLKey: "password",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	pmClient := mockParameterManagerClient(t, &mockParameterManagerServer{
		renderFn: func(ctx context.Context, _ *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
			data := []byte("user: admin\npassword: password@1234")
			return &parametermanagerpb.RenderParameterVersionResponse{
				ParameterVersion: globalParameterVersion2,
				RenderedPayload:  data,
			}, nil
		},
	})
	regionalPmClients := make(map[string]*parametermanager.Client)

	server := &Server{
		ParameterManagerClient:          pmClient,
		RegionalParameterManagerClients: regionalPmClients,
		ServerClientOptions:             []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)

	if !strings.Contains(got.Error(), "Internal") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "both ExtractJSONKey and ExtractYAMLKey can't be simultaneously non empty strings") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

// Even 1 error results in error in mounting
func TestHandleMountEventSMErrorPMVersionOK(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
			{
				ResourceName: globalParameterVersion,
				FileName:     "pm_good1.txt",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "Secret is Disabled")
		},
	})

	pmClient := mockParameterManagerClient(t, &mockParameterManagerServer{
		renderFn: func(ctx context.Context, _ *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
			data := []byte(`user: admin
password: password@1234`)
			return &parametermanagerpb.RenderParameterVersionResponse{
				ParameterVersion: globalParameterVersion2,
				RenderedPayload:  data,
			}, nil
		},
	})

	regionalSmClients := make(map[string]*secretmanager.Client)
	regionalPmClients := make(map[string]*parametermanager.Client)

	server := &Server{
		SecretClient:                    client,
		ParameterManagerClient:          pmClient,
		RegionalSecretClients:           regionalSmClients,
		RegionalParameterManagerClients: regionalPmClients,
		ServerClientOptions:             []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if !strings.Contains(got.Error(), "Internal") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventPMErrorSMVersionOK(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
			{
				ResourceName: globalParameterVersion,
				FileName:     "pm_good1.txt",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("My Secret"),
				},
			}, nil
		},
	})

	pmClient := mockParameterManagerClient(t, &mockParameterManagerServer{
		renderFn: func(ctx context.Context, _ *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "Parameter version is disabled")
		},
	})

	regionalSmClients := make(map[string]*secretmanager.Client)
	regionalPmClients := make(map[string]*parametermanager.Client)

	server := &Server{
		SecretClient:                    client,
		ParameterManagerClient:          pmClient,
		RegionalSecretClients:           regionalSmClients,
		RegionalParameterManagerClients: regionalPmClients,
		ServerClientOptions:             []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if !strings.Contains(got.Error(), "Internal") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventsSMInvalidLocations(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/locations/very_very_very_very_very_very_very_very_long_location/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
			{
				ResourceName: "projects/project/locations/split/location/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mock(t, &mockSecretServer{})
	regionalClients := make(map[string]*secretmanager.Client)
	server := &Server{
		SecretClient:          client,
		RegionalSecretClients: regionalClients,
		ServerClientOptions:   []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if !strings.Contains(got.Error(), "Invalid location") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "unknown resource type") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventsPMInvalidLocations(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/locations/very_very_very_very_very_very_very_very_long_location/parameters/test/versions/parameterVersionId",
				FileName:     "pm_good1.txt",
			},
			{
				ResourceName: "projects/project/locations/split/location/parameters/test/versions/latest",
				FileName:     "good1.txt",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mockParameterManagerClient(t, &mockParameterManagerServer{})
	regionalClients := make(map[string]*parametermanager.Client)
	server := &Server{
		ParameterManagerClient:          client,
		RegionalParameterManagerClients: regionalClients,
		ServerClientOptions:             []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if !strings.Contains(got.Error(), "Invalid location") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "unknown resource type") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventSMMultipleErrors(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test-a/versions/1",
				FileName:     "good1.txt",
			},
			{
				ResourceName: "projects/project/secrets/test-a/versions/2",
				FileName:     "bad1.txt",
			},
			{
				ResourceName: "projects/project/secrets/test-b/versions/latest",
				FileName:     "bad2.txt",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			switch req.Name {
			case "projects/project/secrets/test-a/versions/1":
				return &secretmanagerpb.AccessSecretVersionResponse{
					Name: "projects/project/secrets/test-a/versions/1",
					Payload: &secretmanagerpb.SecretPayload{
						Data: []byte("good data"),
					},
				}, nil
			case "projects/project/secrets/test-a/versions/2":
				return nil, status.Error(codes.FailedPrecondition, "Secret is Disabled")
			case "projects/project/secrets/test-b/versions/latest":
				return nil, status.Error(codes.PermissionDenied, "User does not have permission on secret")
			default:
				return nil, status.Error(codes.NotFound, "Secret not found")
			}
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)

	server := &Server{
		SecretClient:          client,
		RegionalSecretClients: regionalClients,
		ServerClientOptions:   []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if !strings.Contains(got.Error(), "Internal") { // outermost level error
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "PermissionDenied") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventPMMultipleErrors(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: globalParameterVersion,
				FileName:     "pm_good1.txt",
			},
			{
				ResourceName: globalParameterVersion2,
				FileName:     "pm_bad1.txt",
			},
			{
				ResourceName: regionalParameterVersion,
				FileName:     "pm_bad2.txt",
			},
			{
				ResourceName: fmt.Sprintf("%s3", globalParameterVersion),
				FileName:     "pm_bad3.txt",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	client := mockParameterManagerClient(t, &mockParameterManagerServer{
		renderFn: func(ctx context.Context, req *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
			switch req.Name {
			case globalParameterVersion:
				data := []byte("user: admin\npassword: password@1234")
				return &parametermanagerpb.RenderParameterVersionResponse{
					ParameterVersion: globalParameterVersion,
					RenderedPayload:  data,
				}, nil
			case globalParameterVersion2:
				return nil, status.Error(codes.FailedPrecondition, "ParameterVersion is Disabled")
			case regionalParameterVersion:
				return nil, status.Error(codes.PermissionDenied, "User does not have permission on secret")
			default:
				return nil, status.Error(codes.NotFound, "ParameterVersion not found")
			}
		},
	})

	regionalClients := make(map[string]*parametermanager.Client)
	regionalClients["us-central1"] = client

	server := &Server{
		ParameterManagerClient:          client,
		RegionalParameterManagerClients: regionalClients,
		ServerClientOptions:             []option.ClientOption{},
	}
	_, got := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if !strings.Contains(got.Error(), "Internal") { // Outermost level error
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "PermissionDenied") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "NotFound") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventForRegionalSecret(t *testing.T) {
	secretFileMode := int32(0600) // decimal 384
	const secretVersionByAlias = "projects/project/locations/us-central1/secrets/test/versions/latest"
	const secretVersionByID = "projects/project/locations/us-central1/secrets/test/versions/2"

	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: secretVersionByAlias,
				FileName:     "good1.txt",
			},
			{
				ResourceName: secretVersionByAlias,
				FileName:     "good2.txt",
				Mode:         &secretFileMode,
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	want := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{
			{
				Id:      secretVersionByAlias,
				Version: secretVersionByID,
			},
			{
				Id:      secretVersionByAlias,
				Version: secretVersionByID,
			},
		},
		Files: []*v1alpha1.File{
			{
				Path:     "good1.txt",
				Mode:     777,
				Contents: []byte("My Secret"),
			},
			{
				Path:     "good2.txt",
				Mode:     384, // octal 0600
				Contents: []byte("My Secret"),
			},
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: secretVersionByID,
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("Global Secret"),
				},
			}, nil
		},
	})

	regionalClient := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: secretVersionByID,
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("My Secret"),
				},
			}, nil
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)

	regionalClients["us-central1"] = regionalClient

	server := &Server{
		SecretClient:          client,
		RegionalSecretClients: regionalClients,
		ServerClientOptions:   []option.ClientOption{},
	}
	got, err := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}

func TestHandleMountEventForExtractJSONKey(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
			{
				ResourceName:   "projects/project/secrets/test/versions/latest",
				FileName:       "good2.txt",
				ExtractJSONKey: "user",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	want := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{
			{
				Id:      "projects/project/secrets/test/versions/latest",
				Version: "projects/project/secrets/test/versions/2",
			},
			{
				Id:      "projects/project/secrets/test/versions/latest",
				Version: "projects/project/secrets/test/versions/2",
			},
		},
		Files: []*v1alpha1.File{
			{
				Path:     "good1.txt",
				Mode:     777,
				Contents: []byte(`{"user": "admin", "password": "password@1234"}`),
			},
			{
				Path:     "good2.txt",
				Mode:     777,
				Contents: []byte("admin"),
			},
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte(`{"user": "admin", "password": "password@1234"}`),
				},
			}, nil
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)
	server := &Server{
		SecretClient:          client,
		RegionalSecretClients: regionalClients,
		ServerClientOptions:   []option.ClientOption{},
	}
	got, err := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}

func TestHandleMountEventForRegionalSecretExtractJSONKey(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/locations/us-central1/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
			{
				ResourceName:   "projects/project/locations/us-central1/secrets/test/versions/latest",
				FileName:       "good2.txt",
				ExtractJSONKey: "user",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	want := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{
			{
				Id:      "projects/project/locations/us-central1/secrets/test/versions/latest",
				Version: "projects/project/locations/us-central1/secrets/test/versions/2",
			},
			{
				Id:      "projects/project/locations/us-central1/secrets/test/versions/latest",
				Version: "projects/project/locations/us-central1/secrets/test/versions/2",
			},
		},
		Files: []*v1alpha1.File{
			{
				Path:     "good1.txt",
				Mode:     777,
				Contents: []byte(`{"user":"admin", "password":"password@1234"}`),
			},
			{
				Path:     "good2.txt",
				Mode:     777,
				Contents: []byte("admin"),
			},
		},
	}

	regionalClient := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/locations/us-central1/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte(`{"user":"admin", "password":"password@1234"}`),
				},
			}, nil
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)
	regionalClients["us-central1"] = regionalClient

	server := &Server{
		SecretClient:          regionalClient,
		RegionalSecretClients: regionalClients,
		ServerClientOptions:   []option.ClientOption{},
	}

	got, err := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}
func TestHandleMountEventForMultipleSecretsExtractJSONKey(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName:   "projects/project/secrets/test1/versions/latest",
				FileName:       "good1.txt",
				ExtractJSONKey: "user",
			},
			{
				ResourceName:   "projects/project/locations/us-central1/secrets/test2/versions/latest",
				FileName:       "good2.txt",
				ExtractJSONKey: "user",
			},
		},
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	want := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{
			{
				Id:      "projects/project/secrets/test1/versions/latest",
				Version: "projects/project/secrets/test1/versions/2",
			},
			{
				Id:      "projects/project/locations/us-central1/secrets/test2/versions/latest",
				Version: "projects/project/locations/us-central1/secrets/test2/versions/2",
			},
		},
		Files: []*v1alpha1.File{
			{
				Path:     "good1.txt",
				Mode:     777,
				Contents: []byte("admin"),
			},
			{
				Path:     "good2.txt",
				Mode:     777,
				Contents: []byte("admin2"),
			},
		},
	}

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			switch req.Name {
			case "projects/project/secrets/test1/versions/latest":
				return &secretmanagerpb.AccessSecretVersionResponse{
					Name: "projects/project/secrets/test1/versions/2",
					Payload: &secretmanagerpb.SecretPayload{
						Data: []byte(`{"user":"admin", "password":"password@1234"}`),
					},
				}, nil
			case "projects/project/locations/us-central1/secrets/test2/versions/latest":
				return &secretmanagerpb.AccessSecretVersionResponse{
					Name: "projects/project/locations/us-central1/secrets/test2/versions/2",
					Payload: &secretmanagerpb.SecretPayload{
						Data: []byte(`{"user":"admin2", "password":"password@12345"}`),
					},
				}, nil
			default:
				return nil, nil
			}
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)
	regionalClients["us-central1"] = client

	server := &Server{
		SecretClient:          client,
		RegionalSecretClients: regionalClients,
		ServerClientOptions:   []option.ClientOption{},
	}

	got, err := handleMountEvent(context.Background(), NewFakeCreds(), cfg, server)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}

// mock builds a secretmanager.Client talking to a real in-memory secretmanager
// GRPC server of the *mockSecretServer.
func mock(t testing.TB, m *mockSecretServer) *secretmanager.Client {
	t.Helper()
	l := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(s, m)

	go func() {
		if err := s.Serve(l); err != nil {
			t.Errorf("server error: %v", err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:whatever", grpc.WithContextDialer(
		func(context.Context, string) (net.Conn, error) {
			return l.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	client, err := secretmanager.NewClient(context.Background(), option.WithoutAuthentication(), option.WithGRPCConn(conn))
	shutdown := func() {
		t.Log("shutdown called")
		conn.Close()
		s.GracefulStop()
		l.Close()
	}
	if err != nil {
		shutdown()
		t.Fatal(err)
	}

	t.Cleanup(shutdown)
	return client
}

// mockParameterManagerClient builds a parametermanager.Client talking to a real in-memory parametermanager
// GRPC server of the *mockParameterManagerServer.
func mockParameterManagerClient(t testing.TB, m *mockParameterManagerServer) *parametermanager.Client {
	t.Helper()
	l := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	parametermanagerpb.RegisterParameterManagerServer(s, m)

	go func() {
		if err := s.Serve(l); err != nil {
			t.Errorf("server error: %v", err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:whatever", grpc.WithContextDialer(
		func(context.Context, string) (net.Conn, error) {
			return l.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	client, err := parametermanager.NewClient(context.Background(), option.WithoutAuthentication(), option.WithGRPCConn(conn))
	shutdown := func() {
		t.Log("shutdown called")
		conn.Close()
		s.GracefulStop()
		l.Close()
	}
	if err != nil {
		shutdown()
		t.Fatal(err)
	}

	t.Cleanup(shutdown)
	return client
}

// mockSecretServer matches the secremanagerpb.SecretManagerServiceServer
// interface and allows the AccessSecretVersion implementation to be stubbed
// with the accessFn function.
type mockSecretServer struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer
	accessFn func(context.Context, *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error)
}

func (s *mockSecretServer) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if s.accessFn == nil {
		return nil, status.Error(codes.Unimplemented, "mock does not implement accessFn")
	}
	return s.accessFn(ctx, req)
}

// mockParameterManagerServer matches the parametermanagerpb.ParameterManagerServiceServer
// interface and allows the RenderParameterVersion implementation to be stubbed
// with the renderFn function.
type mockParameterManagerServer struct {
	parametermanagerpb.UnimplementedParameterManagerServer
	renderFn func(context.Context, *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error)
}

func (pm *mockParameterManagerServer) RenderParameterVersion(ctx context.Context, req *parametermanagerpb.RenderParameterVersionRequest) (*parametermanagerpb.RenderParameterVersionResponse, error) {
	if pm.renderFn == nil {
		return nil, status.Error(codes.Unimplemented, "mock does not implement renderFn")
	}
	return pm.renderFn(ctx, req)
}

// fakeCreds will adhere to the credentials.PerRPCCredentials interface to add
// empty credentials on a per-rpc basis.
type fakeCreds struct{}

func NewFakeCreds() fakeCreds {
	return fakeCreds{}
}

// GetRequestMetadata gets the request metadata as a map from a TokenSource.
func (f fakeCreds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "fake",
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
// Since these are fake credentials for use with mock local server this is set to false.
func (f fakeCreds) RequireTransportSecurity() bool {
	return false
}
