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

// Package infra holds useful helpers for csi driver server
package infra

import (
	"bytes"
	"context"
	"flag"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

func TestLogInterceptor(t *testing.T) {
	fs := &flag.FlagSet{}
	klog.InitFlags(fs)
	fs.Parse([]string{"-v", "5"})

	klog.LogToStderr(false) // required to make SetOutput work
	b := new(bytes.Buffer)
	klog.SetOutput(b)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{
		Server:     nil,
		FullMethod: "FakeMethod",
	}

	_, got := LogInterceptor()(context.Background(), nil, info, handler)

	if want := codes.OK; status.Code(got) != want {
		t.Errorf("LogInterceptor() error =\n\t%v,\n\twant = %v", got, want)
	}

	klog.Flush()

	if !strings.Contains(b.String(), "request") {
		t.Errorf("LogInterceptor() did not log request\n\tgot:%v", b.String())
	}
	if !strings.Contains(b.String(), "response") {
		t.Errorf("LogInterceptor() did not log response\n\tgot:%v", b.String())
	}
	if !strings.Contains(b.String(), "code=\"OK\"") {
		t.Errorf("LogInterceptor() did not log response code OK, got:\n%v", b.String())
	}
}

func TestLogInterceptor_Error(t *testing.T) {
	fs := &flag.FlagSet{}
	klog.InitFlags(fs)
	fs.Parse([]string{"-v", "5"})

	klog.LogToStderr(false) // required to make SetOutput work
	b := new(bytes.Buffer)
	klog.SetOutput(b)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.Internal, "bad request")
	}
	info := &grpc.UnaryServerInfo{
		Server:     nil,
		FullMethod: "FakeMethod",
	}

	_, got := LogInterceptor()(context.Background(), nil, info, handler)

	if want := codes.Internal; status.Code(got) != want {
		t.Errorf("LogInterceptor() error =\n\t%v,\n\twant = %v", got, want)
	}

	klog.Flush()

	if !strings.Contains(b.String(), "request") {
		t.Errorf("LogInterceptor() did not log request\n\tgot:%v", b.String())
	}
	if !strings.Contains(b.String(), "response") {
		t.Errorf("LogInterceptor() did not log response\n\tgot:%v", b.String())
	}
	if !strings.Contains(b.String(), "code=\"Internal\"") {
		t.Errorf("LogInterceptor() did not log response code Internal, got:\n%v", b.String())
	}
}
