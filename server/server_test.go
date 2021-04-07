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
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

func TestHandleMountEvent(t *testing.T) {
	dir := driveMountHelper(t)

	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
		},
		TargetPath:  dir,
		Permissions: 777,
		PodInfo: &config.PodInfo{
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	want := []byte("My Secret")
	wantMetadata := []*v1alpha1.ObjectVersion{
		{
			Id:      "projects/project/secrets/test/versions/latest",
			Version: "projects/project/secrets/test/versions/2",
		},
	}
	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/secrets/test/versions/2",
				Payload: &secretmanagerpb.SecretPayload{
					Data: want,
				},
			}, nil
		},
	})

	resp, err := handleMountEvent(context.Background(), client, cfg, true)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}

	if diff := cmp.Diff(wantMetadata, resp.GetObjectVersion(), protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}

	got, err := ioutil.ReadFile(filepath.Join(dir, "good1.txt"))
	if err != nil {
		t.Errorf("error reading secret. got err = %v, want err = nil", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("handleMountEvent() wrote unexpected secret value. got = %v, want = %v", got, want)
	}
}

func TestHandleMountEventNoWrite(t *testing.T) {
	dir := driveMountHelper(t)

	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
		},
		TargetPath:  dir,
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
		},
		Files: []*v1alpha1.File{
			{
				Path:     "good1.txt",
				Mode:     777,
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

	got, err := handleMountEvent(context.Background(), client, cfg, false)
	if err != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", err)
	}
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("handleMountEvent() returned unexpected response (-want +got):\n%s", diff)
	}
	if _, err := os.Stat(filepath.Join(dir, "good1.txt")); !os.IsNotExist(err) {
		t.Errorf("handleMountEvent() wrote file good1.txt")
	}
}

func TestHandleMountEventSMError(t *testing.T) {
	dir := driveMountHelper(t)

	cfg := &config.MountConfig{
		Secrets: []*config.Secret{
			{
				ResourceName: "projects/project/secrets/test/versions/latest",
				FileName:     "good1.txt",
			},
		},
		TargetPath:  dir,
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

	_, got := handleMountEvent(context.Background(), client, cfg, true)
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
}

func TestHandleMountEventSMMultipleErrors(t *testing.T) {
	dir := driveMountHelper(t)

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
		TargetPath:  dir,
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

	_, got := handleMountEvent(context.Background(), client, cfg, true)
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if !strings.Contains(got.Error(), "PermissionDenied") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "good1.txt")); !os.IsNotExist(err) {
		t.Errorf("handleMountEvent() wrote file good1.txt")
	}
}

// driveMountHelper creates a temporary directory for use by tests as a
// replacement for the tmpfs directory that the CSI Driver would create for the
// handleMountEvent. This returns the path.
func driveMountHelper(t testing.TB) string {
	t.Helper()
	dir, err := ioutil.TempDir("", "csi-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		log.Printf("Cleaning up: %s", dir)
		os.RemoveAll(dir)
	})
	return dir
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

	conn, err := grpc.Dial(l.Addr().String(), grpc.WithContextDialer(
		func(context.Context, string) (net.Conn, error) {
			return l.Dial()
		}),
		grpc.WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	client, err := secretmanager.NewClient(context.Background(), option.WithGRPCConn(conn))
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
