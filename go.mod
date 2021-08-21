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

module github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp

go 1.16

require (
	cloud.google.com/go v0.93.3
	cloud.google.com/go/secretmanager v0.1.0
	github.com/google/go-cmp v0.5.6
	github.com/googleapis/gax-go/v2 v2.0.5
	go.opentelemetry.io/contrib/instrumentation/runtime v0.16.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.16.0
	golang.org/x/oauth2 v0.0.0-20210805134026-6f1e6394065a
	google.golang.org/api v0.54.0
	google.golang.org/genproto v0.0.0-20210813162853-db860fec028c
	google.golang.org/grpc v1.39.1
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/component-base v0.20.2
	k8s.io/klog/v2 v2.5.0
	sigs.k8s.io/secrets-store-csi-driver v0.0.21
)
