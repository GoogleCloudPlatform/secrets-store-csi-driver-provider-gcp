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
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// handleMountEvent fetches the secrets from the secretmanager API and
// writes them to the filesystem based on the SecretProviderClass configuration.
func handleMountEvent(ctx context.Context, client *secretmanager.Client, cfg *config.MountConfig) error {
	for _, secret := range cfg.Secrets {
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: secret.ResourceName,
		}

		result, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to access secret version (%s): %w", secret.ResourceName, err)
		}

		if err := ioutil.WriteFile(filepath.Join(cfg.TargetPath, secret.FileName), result.Payload.Data, cfg.Permissions); err != nil {
			return fmt.Errorf("failed to write %s at %s: %w", secret.ResourceName, cfg.TargetPath, err)
		}
		log.Printf("secrets-store csi driver wrote %s at %s", secret.ResourceName, cfg.TargetPath)
	}

	return nil
}
