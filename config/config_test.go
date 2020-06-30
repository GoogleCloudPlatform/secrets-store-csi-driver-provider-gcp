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
					"csi.storage.k8s.io/pod.uid": "123"
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
					Namespace: "default",
					Name:      "mypod",
					UID:       "123",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
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
					"csi.storage.k8s.io/pod.uid": "123"
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
					Namespace: "default",
					Name:      "mypod",
					UID:       "123",
				},
				TargetPath:  "/tmp/foo",
				Permissions: 777,
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(tc.in); err == nil {
				t.Errorf("Parse() succeeded for malformed input, want error")
			}
		})
	}
}
