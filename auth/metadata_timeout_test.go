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

package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
)

// blackholeTransport never answers; it waits for the request context to end,
// like a proxy that drops traffic to the metadata server.
type blackholeTransport struct{}

func (blackholeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	<-r.Context().Done()
	return nil, r.Context().Err()
}

func TestGKEWorkloadIdentityMetadataLookupIsBounded(t *testing.T) {
	for _, v := range []string{"PROJECT", "CLUSTER_LOCATION", "CLUSTER_NAME"} {
		t.Setenv(v, "")
	}
	defer func(d time.Duration) { metadataTimeout = d }(metadataTimeout)
	metadataTimeout = 100 * time.Millisecond

	c := &Client{MetadataClient: metadata.NewClient(&http.Client{Transport: blackholeTransport{}})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	_, _, err := c.gkeWorkloadIdentity(ctx, &config.MountConfig{PodInfo: &config.PodInfo{Namespace: "default", Name: "pod"}})
	if err == nil {
		t.Fatal("gkeWorkloadIdentity() err = nil, want an error from the unreachable metadata server")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("gkeWorkloadIdentity() took %v, want it bounded by metadataTimeout", elapsed)
	}
	if ctx.Err() != nil {
		t.Fatalf("mount context ended (%v); the fallback needs it alive", ctx.Err())
	}
}
