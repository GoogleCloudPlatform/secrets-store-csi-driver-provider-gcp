// Copyright 2026 Google LLC
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
	"sync"
	"testing"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// mockClientOptions returns client options that connect any client built from
// them to an in-memory Secret Manager server, so regional clients created by
// the Server during a mount talk to the mock instead of the network.
func mockClientOptions(t testing.TB, m *mockSecretServer) []option.ClientOption {
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
	t.Cleanup(func() {
		conn.Close()
		s.GracefulStop()
		l.Close()
	})
	return []option.ClientOption{option.WithoutAuthentication(), option.WithGRPCConn(conn)}
}

// Mount calls run on separate goroutines and share the regional client cache,
// so creating clients for new locations must be safe under concurrency.
func TestHandleMountConcurrentRegionalSecrets(t *testing.T) {
	opts := mockClientOptions(t, &mockSecretServer{
		accessFn: func(_ context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name:    req.Name,
				Payload: &secretmanagerpb.SecretPayload{Data: []byte("secret for " + req.Name)},
			}, nil
		},
	})

	s := &Server{
		RegionalSecretClients: make(map[string]*secretmanager.Client),
		ServerClientOptions:   opts,
	}
	t.Cleanup(func() {
		for _, c := range s.RegionalSecretClients {
			c.Close()
		}
	})

	const mounts = 32
	errs := make([]error, mounts)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < mounts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("projects/project/locations/region-%d/secrets/test/versions/latest", i)
			cfg := &config.MountConfig{
				Secrets:     []*config.Secret{{ResourceName: name, FileName: "secret.txt"}},
				Permissions: 0644,
				PodInfo:     &config.PodInfo{Namespace: "default", Name: fmt.Sprintf("pod-%d", i)},
			}
			<-start
			got, err := handleMountEvent(context.Background(), NewFakeCreds(), cfg, s)
			if err != nil {
				errs[i] = err
				return
			}
			if want := "secret for " + name; len(got.Files) != 1 || string(got.Files[0].Contents) != want {
				errs[i] = fmt.Errorf("got files %v, want one file with %q", got.Files, want)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("mount %d: %v", i, err)
		}
	}
	if got := len(s.RegionalSecretClients); got != mounts {
		t.Errorf("got %d cached regional clients, want %d", got, mounts)
	}
}
