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

go 1.14

require (
	cloud.google.com/go v0.57.0
	github.com/client9/misspell v0.3.4
	github.com/google/go-cmp v0.4.0
	golang.org/x/tools v0.0.0-20200501065659-ab2804fb9c9d
	google.golang.org/api v0.22.0
	google.golang.org/genproto v0.0.0-20200605102947-12044bf5ea91
	google.golang.org/grpc v1.29.1
	gopkg.in/yaml.v2 v2.3.0
	honnef.co/go/tools v0.0.1-2020.1.3
	k8s.io/apimachinery v0.18.4
)
