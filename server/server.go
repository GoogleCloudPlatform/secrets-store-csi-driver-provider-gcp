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

// Package server implements a grpc server to receive mount events
package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

type Server struct {
	UA             string
	RuntimeVersion string
	Kubeconfig     string // TODO: accept a kubernetes.Clientset instead
}

var _ v1alpha1.CSIDriverProviderServer = &Server{}

// Mount implements provider csi-provider method
func (s *Server) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		log.Printf("Mount() called without a deadline.")
	} else {
		log.Printf("remaining deadline: %v", time.Until(deadline))
	}

	p, err := strconv.ParseUint(req.GetPermission(), 10, 32)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Unable to parse permissions: %s", req.GetPermission()))

	}

	params := &config.MountParams{
		Attributes:  req.GetAttributes(),
		KubeSecrets: req.GetSecrets(),
		TargetPath:  req.GetTargetPath(),
		Permissions: os.FileMode(p),
	}

	cfg, err := config.Parse(params)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	smOpts := []option.ClientOption{option.WithUserAgent(s.UA)}

	if cfg.TokenSource == nil {
		// Build the workload identity auth token
		token, err := auth.Token(ctx, cfg, s.Kubeconfig)
		if err != nil {
			log.Printf("unable to use workload identity: %v", err)
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("Unable to obtain workload identity auth: %v", err))
		} else {
			smOpts = append(smOpts, option.WithTokenSource(oauth2.StaticTokenSource(token)))
		}
	} else {
		// Use the secret provided in the CSI mount command for auth
		smOpts = append(smOpts, option.WithTokenSource(cfg.TokenSource))
	}

	// Build the secret manager client
	client, err := secretmanager.NewClient(ctx, smOpts...)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create secretmanager client: %v", err))
	}

	// Fetch the secrets from the secretmanager API and write them to the
	// filesystem based on the SecretProviderClass configuration.
	ovs, err := handleMountEvent(ctx, client, cfg)

	return &v1alpha1.MountResponse{
		ObjectVersion: ovs,
	}, err
}

// Version implements provider csi-provider method
func (s *Server) Version(ctx context.Context, req *v1alpha1.VersionRequest) (*v1alpha1.VersionResponse, error) {
	return &v1alpha1.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "secrets-store-csi-driver-provider-gcp",
		RuntimeVersion: s.RuntimeVersion,
	}, nil
}

// handleMountEvent fetches the secrets from the secretmanager API and
// writes them to the filesystem based on the SecretProviderClass configuration.
func handleMountEvent(ctx context.Context, client *secretmanager.Client, cfg *config.MountConfig) ([]*v1alpha1.ObjectVersion, error) {
	ovs := []*v1alpha1.ObjectVersion{}
	for _, secret := range cfg.Secrets {
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: secret.ResourceName,
		}

		result, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			// TODO: determine error codes, should we propagate error code space
			// from the secret call to this response?
			log.Printf("failed to access secret version (%s): %s", secret.ResourceName, err)
			return nil, status.Error(codes.Internal, err.Error())
		}

		if err := ioutil.WriteFile(filepath.Join(cfg.TargetPath, secret.FileName), result.Payload.Data, cfg.Permissions); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to write %s at %s: %s", secret.ResourceName, cfg.TargetPath, err))
		}

		log.Printf("secrets-store csi driver wrote %s at %s", secret.ResourceName, cfg.TargetPath)

		ovs = append(ovs, &v1alpha1.ObjectVersion{
			Id:      secret.ResourceName,
			Version: result.GetName(),
		})
	}
	return ovs, nil
}
