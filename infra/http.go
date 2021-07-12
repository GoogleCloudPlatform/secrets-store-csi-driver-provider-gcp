// Copyright 2021 Google LLC
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

// Package infra holds useful helpers for csi driver server plugin
package infra

import (
	"net/http"
	"net/http/httptrace"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
)

// Detailed HTTP
type Detailed struct {
	Transport http.RoundTripper
}

func (f *Detailed) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx := r.Context()
	ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
	return f.Transport.RoundTrip(r.WithContext(ctx))
}

func NewDetailed(base http.RoundTripper) *Detailed {
	return &Detailed{Transport: base}
}
