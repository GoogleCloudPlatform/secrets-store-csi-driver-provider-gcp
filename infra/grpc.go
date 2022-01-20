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
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// LogInterceptor returns a new unary server interceptors that performs request
// and response logging.
func LogInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		deadline, _ := ctx.Deadline()
		dd := time.Until(deadline).String()
		if klog.V(3).Enabled() {
			klog.V(3).InfoS("request", "method", info.FullMethod, "deadline", dd)
		}
		resp, err := handler(ctx, req)
		if klog.V(2).Enabled() {
			s, _ := status.FromError(err)
			klog.V(2).InfoS("response", "method", info.FullMethod, "deadline", dd, "duration", time.Since(start).String(), "status.code", s.Code(), "status.message", s.Message())
		}
		return resp, err
	}
}
