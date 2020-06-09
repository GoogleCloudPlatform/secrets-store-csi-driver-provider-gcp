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

package main

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

	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

var sampleAttrs = `
{
	"secrets": "array:\n  - |\n    resourceName: \"projects/project/secrets/test/versions/latest\"\n    fileName: \"good1.txt\"\n"
}
`

func TestHandleMountEvent(t *testing.T) {
	dir := driveMountHelper(t)

	want := []byte("My Secret")
	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Payload: &secretmanagerpb.SecretPayload{
					Data: want,
				},
			}, nil
		},
	})

	if got := handleMountEvent(context.Background(), client, &mountParams{
		attributes:  sampleAttrs,
		kubeSecrets: "{}",
		targetPath:  dir,
		permissions: 777,
	}); got != nil {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)
	}

	got, err := ioutil.ReadFile(filepath.Join(dir, "good1.txt"))
	if err != nil {
		t.Errorf("error reading secret. got err = %v, want err = nil", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("handleMountEvent() wrote unexpected secret value. got = %v, want = %v", got, want)
	}
}

func TesthandleMountEventSMError(t *testing.T) {
	dir := driveMountHelper(t)

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "Secret is Disabled")
		},
	})

	got := handleMountEvent(context.Background(), client, &mountParams{
		attributes:  sampleAttrs,
		kubeSecrets: "{}",
		targetPath:  dir,
		permissions: 777,
	})
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("handleMountEvent() got err = %v, want err = nil", got)

	}
}

func TestHandleMountEventConfigErrors(t *testing.T) {
	dir := driveMountHelper(t)

	client := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("data"),
				},
			}, nil
		},
	})

	tests := []struct {
		name   string
		params *mountParams
	}{
		{
			name: "unparsable attributes",
			params: &mountParams{
				attributes:  "",
				kubeSecrets: "{}",
				targetPath:  dir,
				permissions: 777,
			},
		},
		{
			name: "missing secrets attribute",
			params: &mountParams{
				attributes:  "{}",
				kubeSecrets: "{}",
				targetPath:  dir,
				permissions: 777,
			},
		},
		{
			name: "unparsable kubernetes secrets",
			params: &mountParams{
				attributes:  sampleAttrs,
				kubeSecrets: "",
				targetPath:  dir,
				permissions: 777,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := handleMountEvent(context.Background(), client, tc.params)
			if got == nil {
				t.Errorf("handleMountEvent() succeeded for malformed input, want error")
			}
		})
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
