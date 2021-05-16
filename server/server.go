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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

type Server struct {
	UA             string
	RuntimeVersion string
	KubeClient     *kubernetes.Clientset
	// WriteSecrets controls whether to write the secrets directly (true) or
	// returns secrets in grpc response (false, requires driver v0.0.21+)
	WriteSecrets bool
}

var _ v1alpha1.CSIDriverProviderServer = &Server{}

// Mount implements provider csi-provider method
func (s *Server) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
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
		token, err := auth.Token(ctx, cfg, s.KubeClient)
		if err != nil {
			klog.ErrorS(err, "unable to use workload identity", "pod", klog.ObjectRef{Namespace: cfg.PodInfo.Namespace, Name: cfg.PodInfo.Name})
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
	return handleMountEvent(ctx, client, cfg, s.WriteSecrets)
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
func handleMountEvent(ctx context.Context, client *secretmanager.Client, cfg *config.MountConfig, writeSecrets bool) (*v1alpha1.MountResponse, error) {
	results := make([]*secretmanagerpb.AccessSecretVersionResponse, len(cfg.Secrets))
	errs := make([]error, len(cfg.Secrets))
	g, ctx := errgroup.WithContext(ctx)

	// In parallel fetch all secrets needed for the mount
	for i, secret := range cfg.Secrets {
		i, secret := i, secret // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			req := &secretmanagerpb.AccessSecretVersionRequest{
				Name: secret.ResourceName,
			}
			resp, err := client.AccessSecretVersion(ctx, req)
			results[i] = resp
			errs[i] = err
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// If any access failed, return a grpc status error that includes each
	// individual status error in the Details field.
	//
	// If there are any failures then there will be no changes to the
	// filesystem. Initial mount events will fail (preventing pod start) and
	// the secrets-store-csi-driver will emit pod events on rotation failures.
	// By erroring out on any failures we prevent partial rotations (i.e. the
	// username file was updated to a new value but the corresponding password
	// field was not).
	if err := buildErr(errs); err != nil {
		return nil, err
	}

	out := &v1alpha1.MountResponse{}

	// Write secrets.
	ovs := make([]*v1alpha1.ObjectVersion, len(cfg.Secrets))
	for i, secret := range cfg.Secrets {
		result := results[i]
		if writeSecrets {
			if err := ioutil.WriteFile(filepath.Join(cfg.TargetPath, secret.FileName), result.Payload.Data, cfg.Permissions); err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("failed to write %s at %s: %s", secret.ResourceName, cfg.TargetPath, err))
			}

			klog.InfoS("wrote secret", "secret", secret.ResourceName, "path", cfg.TargetPath, "pod", klog.ObjectRef{Namespace: cfg.PodInfo.Namespace, Name: cfg.PodInfo.Name})
		} else {
			out.Files = append(out.Files, &v1alpha1.File{
				Path:     secret.FileName,
				Mode:     int32(cfg.Permissions),
				Contents: result.Payload.Data,
			})
			klog.InfoS("added secret to response", "resource_name", secret.ResourceName, "file_name", secret.FileName, "pod", klog.ObjectRef{Namespace: cfg.PodInfo.Namespace, Name: cfg.PodInfo.Name})
		}

		ovs[i] = &v1alpha1.ObjectVersion{
			Id:      secret.ResourceName,
			Version: result.GetName(),
		}
	}
	out.ObjectVersion = ovs

	return out, nil
}

// buildErr consolidates many errors into a single Status protobuf error message
// with each individual error included into the status Details any proto. The
// consolidated proto is converted to a general error.
func buildErr(errs []error) error {
	msgs := make([]string, 0, len(errs))
	hasErr := false
	s := &spb.Status{
		Code:    int32(codes.Internal),
		Details: make([]*anypb.Any, 0),
	}

	for i := range errs {
		if errs[i] == nil {
			continue
		}
		hasErr = true
		msgs = append(msgs, errs[i].Error())

		any, _ := anypb.New(status.Convert(errs[i]).Proto())
		s.Details = append(s.Details, any)
	}
	if !hasErr {
		return nil
	}
	s.Message = strings.Join(msgs, ",")
	return status.FromProto(s).Err()
}
