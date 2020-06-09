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
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	yaml "gopkg.in/yaml.v2"
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

// StringArray holds the 'secrets' key of the SecretProviderClass parameters.
// This is an array of yaml strings. Each string then must be parsed as a
// 'Secret' struct.
type StringArray struct {
	Array []string `json:"array" yaml:"array"`
}

// mountParams hold information from the CSI Driver about the mount event.
type mountParams struct {
	attributes  string
	kubeSecrets string
	targetPath  string
	permissions os.FileMode
}

// handleMountEvent fetches the secrets from the secretmanager API and
// writes them to the filesystem based on the SecretProviderClass configuration.
func handleMountEvent(ctx context.Context, client *secretmanager.Client, params *mountParams) error {
	var attrib, secret map[string]string

	// Everything in the "parameters" section of the SecretProviderClass.
	if err := json.Unmarshal([]byte(params.attributes), &attrib); err != nil {
		return fmt.Errorf("failed to unmarshal attributes: %v", err)
	}

	// The secrets here are the relevant CSI driver (k8s) secrets. See
	// https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html
	// Currently unused.
	if err := json.Unmarshal([]byte(params.kubeSecrets), &secret); err != nil {
		return fmt.Errorf("failed to unmarshal secrets: %v", err)
	}

	// TODO(#4): redact attributes + secrets (or make configurable)
	log.Printf("attributes: %v", attrib)
	log.Printf("secrets: %v", secret)
	log.Printf("filePermission: %v", params.permissions)
	log.Printf("targetPath: %v", params.targetPath)

	var objects StringArray
	if _, ok := attrib["secrets"]; !ok {
		return errors.New("missing required 'secrets' attribute")
	}
	if err := yaml.Unmarshal([]byte(attrib["secrets"]), &objects); err != nil {
		return fmt.Errorf("failed to unmarshal secrets attribute: %v", err)
	}

	for i, object := range objects.Array {
		var secret Secret
		if err := yaml.Unmarshal([]byte(object), &secret); err != nil {
			return fmt.Errorf("failed to unmarshal secret at index %d: %v", i, err)
		}

		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: secret.ResourceName,
		}

		result, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to access secret version (%s): %w", secret.ResourceName, err)
		}

		if err := ioutil.WriteFile(filepath.Join(params.targetPath, secret.FileName), result.Payload.Data, params.permissions); err != nil {
			return fmt.Errorf("failed to write %s at %s: %w", secret.ResourceName, params.targetPath, err)
		}
		log.Printf("secrets-store csi driver wrote %s at %s", secret.ResourceName, params.targetPath)
	}

	return nil
}
