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
	"io"
	"net/http"
	"strings"
	"testing"
)

// trackedBody records whether the response body was closed.
type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

// fixedResponse answers every request with the same status and body.
type fixedResponse struct {
	status int
	body   *trackedBody
}

func (f *fixedResponse) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       f.body,
		Header:     make(http.Header),
	}, nil
}

func TestTradeIDBindTokenClosesBody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"ok", http.StatusOK, `{"access_token":"token","token_type":"Bearer"}`, ""},
		{"forbidden", http.StatusForbidden, `{"error":"denied"}`, "could not get idbindtoken token, status: 403"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := &fixedResponse{status: tc.status, body: &trackedBody{Reader: strings.NewReader(tc.body)}}
			client := &http.Client{Transport: rt}

			tok, err := tradeIDBindToken(context.Background(), client, "k8s-token", "audience")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("tradeIDBindToken() err = %v, want nil", err)
				}
				if tok.AccessToken != "token" {
					t.Errorf("tradeIDBindToken() access token = %q, want %q", tok.AccessToken, "token")
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("tradeIDBindToken() err = %v, want %q", err, tc.wantErr)
			}
			if !rt.body.closed {
				t.Error("tradeIDBindToken() left the response body open")
			}
		})
	}
}
