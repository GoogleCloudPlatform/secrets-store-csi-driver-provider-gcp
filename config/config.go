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

// Package config includes helpers for parsing and accessing the information
// from the secrets CSI driver mount events.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const (
	attributePodName            = "csi.storage.k8s.io/pod.name"
	attributePodNamespace       = "csi.storage.k8s.io/pod.namespace"
	attributePodUID             = "csi.storage.k8s.io/pod.uid"
	attributeServiceAccountName = "csi.storage.k8s.io/serviceAccount.name"
)

// Secret holds the parameters of the SecretProviderClass CRD. Links the GCP
// secret resource name to a path in the filesystem.
type Secret struct {
	// ResourceName refers to a SecretVersion in the format
	// projects/*/secrets/*/versions/*.
	ResourceName string `json:"resourceName" yaml:"resourceName"`

	// FileName is where the contents of the secret are to be written.
	FileName string `json:"fileName" yaml:"fileName"`

	// Path is the relative path where the contents of the secret are written.
	Path string `json:"path" yaml:"path"`

	// Mode is the optional file mode for the file containing the secret. Must be
	// an octal value between 0000 and 0777 or a decimal value between 0 and 511
	Mode *int32 `json:"mode,omitempty" yaml:"mode,omitempty"`
}

// PodInfo includes details about the pod that is receiving the mount event.
type PodInfo struct {
	Namespace      string
	Name           string
	UID            types.UID
	ServiceAccount string
}

// MountConfig holds the parsed information from a mount event.
type MountConfig struct {
	Secrets     []*Secret
	PodInfo     *PodInfo
	TargetPath  string
	Permissions os.FileMode
	// AuthPodADC identifies whether Workload Identity should be used for
	// authentication. This is the of the pod for volume mount (default)
	AuthPodADC bool
	// AuthProviderADC identifies whether the Application Default Credentials of the
	// GCP Provider DaemonSet should be used for authentication.
	// https://cloud.google.com/docs/authentication/production#automatically
	AuthProviderADC bool
	// AuthNodePublishSecret identifies whether the a K8s Secret provided on the
	// NodePublish call should be used for authentication.
	// https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html
	//
	// If set then AuthKubeSecret will contain the json representation of the
	// Google credential (parseable by google.CredentialsFromJSON).
	AuthNodePublishSecret bool
	AuthKubeSecret        []byte
}

// MountParams hold unparsed arguments from the CSI Driver from the mount event.
type MountParams struct {
	Attributes  string
	KubeSecrets string
	TargetPath  string
	Permissions os.FileMode
}

// PathString returns either the FileName or Path parameter of the Secret.
func (s *Secret) PathString() string {
	if s.Path != "" {
		return s.Path
	}
	return s.FileName
}

// Parse parses the input MountParams to the more structured MountConfig.
func Parse(in *MountParams) (*MountConfig, error) {
	out := &MountConfig{}
	out.Permissions = in.Permissions
	out.TargetPath = in.TargetPath
	out.Secrets = make([]*Secret, 0)

	var attrib, secret map[string]string

	// Everything in the "parameters" section of the SecretProviderClass.
	if err := json.Unmarshal([]byte(in.Attributes), &attrib); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes: %v", err)
	}

	out.PodInfo = &PodInfo{
		Namespace:      attrib[attributePodNamespace],
		Name:           attrib[attributePodName],
		UID:            types.UID(attrib[attributePodUID]),
		ServiceAccount: attrib[attributeServiceAccountName],
	}

	podInfo := klog.ObjectRef{Namespace: out.PodInfo.Namespace, Name: out.PodInfo.Name}

	// The secrets here are the relevant CSI driver (k8s) secrets. See
	// https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html
	if err := json.Unmarshal([]byte(in.KubeSecrets), &secret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %v", err)
	}
	if _, ok := secret["key.json"]; ok {
		out.AuthNodePublishSecret = true
		out.AuthKubeSecret = []byte(secret["key.json"])
	}

	switch attrib["auth"] {
	case "provider-adc":
		if out.AuthNodePublishSecret {
			klog.InfoS("attempting to set both nodePublishSecretRef and provider-adc auth. For details consult https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/blob/main/docs/authentication.md", "pod", podInfo)
			return nil, fmt.Errorf("attempting to set both nodePublishSecretRef and provider-adc auth")
		}
		out.AuthProviderADC = true
	case "pod-adc":
		if out.AuthNodePublishSecret {
			klog.InfoS("attempting to set both nodePublishSecretRef and pod-adc auth. For details consult https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/blob/main/docs/authentication.md", "pod", podInfo)
			return nil, fmt.Errorf("attempting to set both nodePublishSecretRef and pod-adc auth")
		}
		out.AuthPodADC = true
	case "":
		// default to pod auth unless nodePublishSecret is set
		out.AuthPodADC = !out.AuthNodePublishSecret
	default:
		klog.InfoS("unknown auth configuration", "pod", podInfo)
		return nil, fmt.Errorf("unknown auth configuration: %q", attrib["auth"])
	}

	if out.AuthNodePublishSecret {
		klog.V(3).InfoS("parsed auth", "auth", "nodePublishSecretRef", "pod", podInfo)
	}
	if out.AuthPodADC {
		klog.V(3).InfoS("parsed auth", "auth", "pod-adc", "pod", podInfo)
	}
	if out.AuthProviderADC {
		klog.V(3).InfoS("parsed auth", "auth", "provider-adc", "pod", podInfo)
	}

	if os.Getenv("DEBUG") == "true" {
		klog.V(5).InfoS(fmt.Sprintf("attributes: %v", attrib), "pod", podInfo)
		klog.V(5).InfoS(fmt.Sprintf("secrets: %v", secret), "pod", podInfo)
	} else {
		klog.V(5).InfoS("attributes: REDACTED (envvar DEBUG=true to see values)", "pod", podInfo)
		klog.V(5).InfoS("secrets: REDACTED (envvar DEBUG=true to see values)", "pod", podInfo)
	}
	klog.V(5).InfoS(fmt.Sprintf("filePermission: %v", in.Permissions), "pod", podInfo)
	klog.V(5).InfoS(fmt.Sprintf("targetPath: %v", in.TargetPath), "pod", podInfo)

	if _, ok := attrib["secrets"]; !ok {
		return nil, errors.New("missing required 'secrets' attribute")
	}
	if err := yaml.Unmarshal([]byte(attrib["secrets"]), &out.Secrets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets attribute: %v", err)
	}

	return out, nil
}
