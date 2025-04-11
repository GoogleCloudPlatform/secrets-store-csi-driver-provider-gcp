// Copyright 2020 Google LLC
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

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

func TestHandleMountEvent(t *testing.T) {
	secretFileMode := int32(0600) // decimal 384

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
				Name: "projects/project/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("My Secret"),
				},
			}, nil
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)

	got, err := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}

func TestHandleMountEventSMError(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
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

	regionalClients := make(map[string]*secretmanager.Client)
	_, got := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventsInvalidLocations(t *testing.T) {
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
	_, got := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
	if !strings.Contains(got.Error(), "invalid location") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "Invalid secret resource name") {
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
				return nil, status.Error(codes.FailedPrecondition, "Secret is Disabled")
			}
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)

	_, got := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "PermissionDenied") {
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

	got, err := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
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

	got, err := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
}

func TestHandleMountEventForExtractJSONKeyNestedFormatError(t *testing.T) {
	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName:   "projects/project/secrets/test/versions/latest",
				FileName:       "good.txt",
				ExtractJSONKey: "devEnv",
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
					Data: []byte(`{"devEnv": {"user": "admin", "password": "password@1234"}}`),
				},
			}, nil
		},
	})

	regionalClients := make(map[string]*secretmanager.Client)

	_, got := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
	if !strings.Contains(got.Error(), "wrong type for content, expected string but got") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
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

	got, err := handleMountEvent(context.Background(), regionalClient, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
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

	got, err := handleMountEvent(context.Background(), client, NewFakeCreds(), cfg, regionalClients, []option.ClientOption{})
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
