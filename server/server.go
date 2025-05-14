// Copyright 2025 Google LLC
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
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/util"
	"github.com/googleapis/gax-go/v2"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"google.golang.org/api/option"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

type Server struct {
	RuntimeVersion                  string
	AuthClient                      *auth.Client
	SecretClient                    *secretmanager.Client
	ParameterManagerClient          *parametermanager.Client
	RegionalSecretClients           map[string]*secretmanager.Client
	RegionalParameterManagerClients map[string]*parametermanager.Client
	ServerClientOptions             []option.ClientOption
}

// Keeping it separate as same resource name can be used to
// mount at 2 different locations (maybe in different modes for different permissions)
type resourceIdentity struct {
	ResourceName string
	FileName     string
	Path         string
}

var _ v1alpha1.CSIDriverProviderServer = &Server{}
var _ resourceFetcherInterface = &resourceFetcher{}

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

	ts, err := s.AuthClient.TokenSource(ctx, cfg)
	if err != nil {
		klog.ErrorS(err, "unable to obtain auth for mount", "pod", klog.ObjectRef{Namespace: cfg.PodInfo.Namespace, Name: cfg.PodInfo.Name})
		return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("unable to obtain auth for mount: %v", err))
	}

	// Build a grpc credentials.PerRPCCredentials using
	// the grpc google.golang.org/grpc/credentials/oauth package, not to be
	// confused with the oauth2.TokenSource that it wraps.
	gts := oauth.TokenSource{TokenSource: ts}

	// Fetch the secrets from the secretmanager API based on the
	// SecretProviderClass configuration.
	return handleMountEvent(ctx, gts, cfg, s)
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
// include them in the MountResponse based on the SecretProviderClass
// configuration.
func handleMountEvent(ctx context.Context, creds credentials.PerRPCCredentials, cfg *config.MountConfig, s *Server) (*v1alpha1.MountResponse, error) {
	// need to build a per-rpc call option based of the tokensource
	callAuth := gax.WithGRPCOptions(grpc.PerRPCCredentials(creds))

	// Storing it as a resultMap to have 1 API call for each resource instead
	// of de-duplicating API calls for duplicate resources
	resultMap := make(map[resourceIdentity]*Resource)

	for _, secret := range cfg.Secrets {
		if util.IsSecretResource(secret.ResourceName) {
			location, err := util.ExtractLocationFromSecretResource(secret.ResourceName)
			if err != nil {
				resultMap[resourceIdentity{secret.ResourceName, secret.FileName, secret.Path}] = getErrorResource(secret.ResourceName, secret.FileName, secret.Path, err)
				continue
			}
			_, ok := s.RegionalSecretClients[location]
			if !ok {
				s.RegionalSecretClients[location] = util.GetRegionalSecretManagerClient(ctx, location, s.ServerClientOptions)
			}
		} else if util.IsParameterManagerResource(secret.ResourceName) {
			location, err := util.ExtractLocationFromParameterManagerResource(secret.ResourceName)
			if err != nil {
				resultMap[resourceIdentity{secret.ResourceName, secret.FileName, secret.Path}] = getErrorResource(secret.ResourceName, secret.FileName, secret.Path, err)
				continue
			}
			_, ok := s.RegionalParameterManagerClients[location]
			if !ok {
				s.RegionalParameterManagerClients[location] = util.GetRegionalParameterManagerClient(ctx, location, s.ServerClientOptions)
			}
		} else {
			resultMap[resourceIdentity{secret.ResourceName, secret.FileName, secret.Path}] = getErrorResource(secret.ResourceName, secret.FileName, secret.Path, fmt.Errorf("unknown resource type"))
		}
	}
	// In parallel fetch all secrets needed for the mount
	wg := sync.WaitGroup{}
	outputChannel := make(chan *Resource, len(cfg.Secrets))
	for _, secret := range cfg.Secrets {
		if val, ok := resultMap[resourceIdentity{secret.ResourceName, secret.FileName, secret.Path}]; ok && val.Err != nil {
			klog.ErrorS(val.Err, "error for resourceName: ", secret.ResourceName, val.Err)
			continue
		}
		wg.Add(1)
		resourceFetcher := &resourceFetcher{
			ResourceURI:    secret.ResourceName,
			FileName:       secret.FileName,
			Path:           secret.Path,
			ExtractJSONKey: secret.ExtractJSONKey,
			ExtractYAMLKey: secret.ExtractYAMLKey,
		}
		go resourceFetcher.Orchestrator(ctx, s, &callAuth, outputChannel, &wg)
	}
	wg.Wait()
	close(outputChannel)
	for item := range outputChannel {
		if item.Err != nil {
			klog.ErrorS(item.Err, "failed to fetch secret", "resource_name", item.ID)
		}
		resultMap[resourceIdentity{item.ID, item.FileName, item.Path}] = item

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

	if err := buildErr(resultMap); err != nil {
		return nil, err
	}

	out := &v1alpha1.MountResponse{}

	// Add secrets to response.
	ovs := make([]*v1alpha1.ObjectVersion, len(cfg.Secrets))

	if cfg.Permissions > math.MaxInt32 {
		return nil, fmt.Errorf("invalid file permission %d", cfg.Permissions)
	}
	for i, secret := range cfg.Secrets {
		// #nosec G115 Checking limit
		mode := int32(cfg.Permissions)
		if secret.Mode != nil {
			mode = *secret.Mode
		}
		resourceKey := resourceIdentity{secret.ResourceName, secret.FileName, secret.Path}
		resource, ok := resultMap[resourceKey]

		// Should ideally never hit this if block
		if !ok || resource == nil {
			// This indicates a goroutine panicked without sending to outputChannel,
			// and no pre-existing error was recorded in resultMap during client/location checks.
			return nil, status.Error(codes.Internal, fmt.Sprintf("internal error: result missing for secret %v (file: %v, path: %v)", secret.ResourceName, secret.FileName, secret.Path))
		}

		out.Files = append(out.Files, &v1alpha1.File{
			Path:     secret.PathString(),
			Mode:     mode,
			Contents: resource.Payload,
		})
		klog.V(5).InfoS("added secret to response", "resource_name", secret.ResourceName, "file_name", secret.FileName, "pod", klog.ObjectRef{Namespace: cfg.PodInfo.Namespace, Name: cfg.PodInfo.Name})

		// Id:      "projects/project/secrets/test/versions/latest",
		// Version: "projects/project/secrets/test/versions/2",
		// Id and Version will differ only for secret manager results.
		// They will be the same for parameter manager
		ovs[i] = &v1alpha1.ObjectVersion{
			Id:      secret.ResourceName,
			Version: resource.Version,
		}
	}
	out.ObjectVersion = ovs
	return out, nil
}

// buildErr consolidates many errors into a single Status protobuf error message
// with each individual error included into the status Details any proto. The
// consolidated proto is converted to a general error.
func buildErr(resultMap map[resourceIdentity]*Resource) error {
	msgs := make([]string, 0, len(resultMap))
	hasErr := false
	s := &spb.Status{
		Code:    int32(codes.Internal),
		Details: make([]*anypb.Any, 0),
	}
	for name, resource := range resultMap {
		if resource.Err != nil {
			hasErr = true
			msgs = append(msgs, fmt.Sprintf("%s: %s", name, resource.Err.Error()))

			any, _ := anypb.New(status.Convert(resource.Err).Proto())
			s.Details = append(s.Details, any)
		}
	}
	if !hasErr {
		return nil
	}
	s.Message = strings.Join(msgs, ",")
	return status.FromProto(s).Err()
}
