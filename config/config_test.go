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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		in   *MountParams
		want *MountConfig
	}{
		{
			name: "single secret",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
				AuthPodADC:  true,
			},
		},
		{
			name: "single secret with mode",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n  mode: 0600\n",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
						Mode:         int32Ptr(384), // octal 0600
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
				AuthPodADC:  true,
			},
		},
		{
			name: "multiple secret",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n- resourceName: \"projects/project/secrets/test2/versions/latest\"\n  fileName: \"good2.txt\"\n",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
					},
					{
						ResourceName: "projects/project/secrets/test2/versions/latest",
						FileName:     "good2.txt",
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
				AuthPodADC:  true,
			},
		},
		{
			name: "multiple secrets with decimal and octal modes",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n  mode: 256\n- resourceName: \"projects/project/secrets/test2/versions/latest\"\n  fileName: \"good2.txt\"\n  mode: 0600\n",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
						Mode:         int32Ptr(256), // octal 0400
					},
					{
						ResourceName: "projects/project/secrets/test2/versions/latest",
						FileName:     "good2.txt",
						Mode:         int32Ptr(384), // octal 0600
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
				AuthPodADC:  true,
			},
		},
		{
			name: "nodePublishSecretRef",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: `{"key.json":"{\"private_key_id\": \"123\",\"private_key\": \"a-secret\",\"token_uri\": \"https://example.com/token\",\"type\": \"service_account\"}"}`,
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:            "/tmp/foo",
				Permissions:           777,
				AuthNodePublishSecret: true,
				AuthKubeSecret:        []byte(`{"private_key_id": "123","private_key": "a-secret","token_uri": "https://example.com/token","type": "service_account"}`),
			},
		},
		{
			name: "Provider ADC auth",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"auth": "provider-adc",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:      "/tmp/foo",
				Permissions:     777,
				AuthProviderADC: true,
			},
		},
		{
			name: "Pod ADC auth",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"auth": "pod-adc",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
			want: &MountConfig{
				Secrets: []*Secret{
					{
						ResourceName: "projects/project/secrets/test/versions/latest",
						FileName:     "good1.txt",
					},
				},
				PodInfo: &PodInfo{
					Namespace:      "default",
					Name:           "mypod",
					UID:            "123",
					ServiceAccount: "mysa",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
				AuthPodADC:  true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Errorf("Parse() failed: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ParseAccessString() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   *MountParams
	}{
		{
			name: "unparsable attributes",
			in: &MountParams{
				Attributes:  "",
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
		{
			name: "missing secrets attribute",
			in: &MountParams{
				Attributes:  "{}",
				KubeSecrets: "{}",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
		{
			name: "unparsable secrets mode",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n  mode: \"-rw-------\"",
				}
				`,
				KubeSecrets: "",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
		{
			name: "unparsable kubernetes secrets",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123"
				}
				`,
				KubeSecrets: "",
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
		{
			name: "both nodePublishSecretRef and provider-adc",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"auth": "provider-adc",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: `{"key.json":"{\"private_key_id\": \"123\",\"private_key\": \"a-secret\",\"token_uri\": \"https://example.com/token\",\"type\": \"service_account\"}"}`,
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
		{
			name: "both nodePublishSecretRef and pod-adc",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"auth": "pod-adc",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: `{"key.json":"{\"private_key_id\": \"123\",\"private_key\": \"a-secret\",\"token_uri\": \"https://example.com/token\",\"type\": \"service_account\"}"}`,
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
		{
			name: "unknown auth",
			in: &MountParams{
				Attributes: `
				{
					"secrets": "- resourceName: \"projects/project/secrets/test/versions/latest\"\n  fileName: \"good1.txt\"\n",
					"auth": "super-good-auth",
					"csi.storage.k8s.io/pod.namespace": "default",
					"csi.storage.k8s.io/pod.name": "mypod",
					"csi.storage.k8s.io/pod.uid": "123",
					"csi.storage.k8s.io/serviceAccount.name": "mysa"
				}
				`,
				KubeSecrets: `{"key.json":"{\"private_key_id\": \"123\",\"private_key\": \"a-secret\",\"token_uri\": \"https://example.com/token\",\"type\": \"service_account\"}"}`,
				TargetPath:  "/tmp/foo",
				Permissions: 777,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.in); err == nil {
				t.Errorf("Parse() succeeded for malformed input, want error")
			}
		})
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
