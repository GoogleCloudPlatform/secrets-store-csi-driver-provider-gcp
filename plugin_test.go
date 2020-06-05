package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

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

func TestPlugin(t *testing.T) {
	dir, cleanup := driveMountHelper(t)
	defer cleanup()

	want := []byte("My Secret")
	client, shutdown := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Payload: &secretmanagerpb.SecretPayload{
					Data: want,
				},
			}, nil
		},
	})
	defer shutdown()

	if got := plugin(context.Background(), client, sampleAttrs, "{}", dir, "777"); got != nil {
		t.Errorf("plugin() got err = %v, want err = nil", got)
	}

	got, err := ioutil.ReadFile(path.Join(dir, "good1.txt"))
	if err != nil {
		t.Errorf("error reading secret. got err = %v, want err = nil", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("plugin() wrote unexpected secret value. got = %v, want = %v", got, want)
	}
}

func TestPluginSMError(t *testing.T) {
	dir, cleanup := driveMountHelper(t)
	defer cleanup()

	client, shutdown := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "Secret is Disabled")
		},
	})
	defer shutdown()

	got := plugin(context.Background(), client, sampleAttrs, "{}", dir, "777")
	if !strings.Contains(got.Error(), "FailedPrecondition") {
		t.Errorf("plugin() got err = %v, want err = nil", got)

	}
}

func TestPluginConfigErrors(t *testing.T) {
	dir, cleanup := driveMountHelper(t)
	defer cleanup()

	client, shutdown := mock(t, &mockSecretServer{
		accessFn: func(ctx context.Context, _ *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("data"),
				},
			}, nil
		},
	})
	defer shutdown()

	tests := []struct {
		name    string
		attrs   string
		secrets string
		dir     string
		perm    string
	}{
		{
			name:    "unparsable attributes",
			attrs:   "",
			secrets: "{}",
			dir:     dir,
			perm:    "777",
		},
		{
			name:    "unparsable secrets",
			attrs:   sampleAttrs,
			secrets: "",
			dir:     dir,
			perm:    "777",
		},
		{
			name:    "unparsable perm",
			attrs:   sampleAttrs,
			secrets: "{}",
			dir:     dir,
			perm:    "foobar",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := plugin(context.Background(), client, tc.attrs, tc.secrets, tc.dir, tc.perm)
			if got == nil {
				t.Errorf("plugin() succeeded for malformed input, want error")
			}
		})
	}
}

// driveMountHelper creates a temporary directory for use by tests as a
// replacement for the tmpfs directory that the CSI Driver would create for the
// plugin. This returns the path and a cleanup function to run at the end of
// the test.
func driveMountHelper(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := ioutil.TempDir("", "csi-test")
	if err != nil {
		t.Fatal(err)
	}
	return dir, func() {
		log.Printf("Cleaning up: %s", dir)
		os.RemoveAll(dir)
	}
}

// mock builds a secretmanager.Client talking to a real in-memory secretmanager
// GRPC server of the *mockSecretServer. It also returns a function that must
// be called to shut down the grpc server gracefully.
func mock(t *testing.T, m *mockSecretServer) (*secretmanager.Client, func()) {
	t.Helper()
	l := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(s, m)

	go func() {
		if err := s.Serve(l); err != nil {
			t.Logf("server error: %v", err)
		}
	}()

	conn, err := grpc.Dial(l.Addr().String(), grpc.WithDialer(func(string, time.Duration) (net.Conn, error) { return l.Dial() }), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	client, err := secretmanager.NewClient(context.Background(), option.WithGRPCConn(conn))
	shutdown := func() {
		log.Println("shutdown called")
		conn.Close()
		s.Stop()
		l.Close()
	}
	if err != nil {
		shutdown()
		t.Fatal(err)
	}
	return client, shutdown
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
