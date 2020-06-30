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
	"log"
	"os"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/types"
)

// Secret holds the parameters of the SecretProviderClass CRD. Links the GCP
// secret resource name to a path in the filesystem.
type Secret struct {
	// ResourceName refers to a SecretVersion in the format
	// projects/*/secrets/*/versions/*.
	ResourceName string `json:"resourceName" yaml:"resourceName"`

	// FileName is where the contents of the secret are to be written.
	FileName string `json:"fileName" yaml:"fileName"`
}

// PodInfo includes details about the pod that is receiving the mount event.
type PodInfo struct {
	Namespace string
	Name      string
	UID       types.UID
}

// MountConfig holds the parsed information from a mount event.
type MountConfig struct {
	Secrets     []*Secret
	PodInfo     *PodInfo
	TargetPath  string
	Permissions os.FileMode
}

// stringArray holds the 'secrets' key of the SecretProviderClass parameters.
// This is an array of yaml strings. Each string then must be parsed as a
// 'Secret' struct.
type stringArray struct {
	Array []string `json:"array" yaml:"array"`
}

// MountParams hold unparsed arguments from the CSI Driver from the mount event.
type MountParams struct {
	Attributes  string
	KubeSecrets string
	TargetPath  string
	Permissions os.FileMode
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
		Namespace: attrib["csi.storage.k8s.io/pod.namespace"],
		Name:      attrib["csi.storage.k8s.io/pod.name"],
		UID:       types.UID(attrib["csi.storage.k8s.io/pod.uid"]),
	}

	// The secrets here are the relevant CSI driver (k8s) secrets. See
	// https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html
	// Currently unused.
	if err := json.Unmarshal([]byte(in.KubeSecrets), &secret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %v", err)
	}

	// TODO(#4): redact attributes + secrets (or make configurable)
	log.Printf("attributes: %v", attrib)
	log.Printf("secrets: %v", secret)
	log.Printf("filePermission: %v", in.Permissions)
	log.Printf("targetPath: %v", in.TargetPath)

	var objects stringArray
	if _, ok := attrib["secrets"]; !ok {
		return nil, errors.New("missing required 'secrets' attribute")
	}
	if err := yaml.Unmarshal([]byte(attrib["secrets"]), &objects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets attribute: %v", err)
	}

	for i, object := range objects.Array {
		var secret Secret
		if err := yaml.Unmarshal([]byte(object), &secret); err != nil {
			return nil, fmt.Errorf("failed to unmarshal secret at index %d: %v", i, err)
		}
		out.Secrets = append(out.Secrets, &secret)
	}

	return out, nil
}
