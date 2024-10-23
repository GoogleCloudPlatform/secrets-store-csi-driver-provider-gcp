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
	"hash/crc32"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/csrmetrics"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/iterator"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
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

func fetchSecrets(ctx context.Context, projectID string, version string, labels map[string]string, regionalClients map[string]*secretmanager.Client, callAuth gax.CallOption, smOpts []option.ClientOption) ([]*secretmanagerpb.AccessSecretVersionResponse, error) {
	var parent string
	var filter string
	location, ok := labels["location"]
	var secretClient *secretmanager.Client

	if ok {
		// Remove location from labels
		delete(labels, "location")
		parent = fmt.Sprintf("projects/%s/locations/%s", projectID, location)
		filter = buildFilter(labels)

		if _, ok := regionalClients[location]; !ok {
			ep := option.WithEndpoint(fmt.Sprintf("secretmanager.%s.rep.googleapis.com:443", location))
			regionalClient, err := secretmanager.NewClient(ctx, append(smOpts, ep)...)
			if err != nil {
				return nil, err
			}
			regionalClients[location] = regionalClient
		}
		secretClient = regionalClients[location]
	} else {
		parent = fmt.Sprintf("projects/%s", projectID)
		filter = buildFilter(labels)
		client, err := secretmanager.NewClient(ctx, smOpts...)
		if err != nil {
			return nil, err
		}
		secretClient = client
	}
	klog.InfoS("fetching secrets", "parent", parent, "filter", filter)

	req := &secretmanagerpb.ListSecretsRequest{
		Parent: parent,
		Filter: filter,
	}

	var secrets []*secretmanagerpb.Secret

	it := secretClient.ListSecrets(ctx, req, callAuth)

	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		secrets = append(secrets, resp)
	}
	klog.InfoS("Total secrets fetched", "count", fmt.Sprintf("%d", len(secrets)))

	// Optimal limit for max number of secrets can be fetched without causing CSI Secret Store Driver to behave abnormally
	if len(secrets) > 1000 {
		err := status.Error(codes.Internal, "too many secrets")
		klog.ErrorS(err, "Too many secrets found while filtering", "count", fmt.Sprintf("%d", len(secrets)))
		return nil, err
	}

	if len(secrets) == 0 {
		err := status.Error(codes.Internal, "No secrets matched.")
		klog.ErrorS(err, "No secrets matched while filtering", "count", fmt.Sprintf("%d", len(secrets)))
		return nil, err
	}

	var result []*secretmanagerpb.AccessSecretVersionResponse
	for _, secret := range secrets {
		versionReq := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("%s/versions/%s", secret.Name, version),
		}
		versionResp, err := secretClient.AccessSecretVersion(ctx, versionReq, callAuth)
		if err != nil {
			// If the version is disabled, destroyed, or does not exist, skip it
			continue
		}
		crc32c := crc32.MakeTable(crc32.Castagnoli)
		checksum := int64(crc32.Checksum(versionResp.Payload.Data, crc32c))
		if checksum != *versionResp.Payload.DataCrc32C {
			err := status.Error(codes.Internal, "Data corruption detected.")
			klog.ErrorS(err, "Secret value is corrupted", "secret", secret.Name, "version", version)
			continue
		}
		result = append(result, versionResp)
	}
	return result, nil
}

func buildFilter(labels map[string]string) string {
	var filterParts []string
	for key, value := range labels {
		filterParts = append(filterParts, fmt.Sprintf("labels.%s=%s", key, value))
	}
	return strings.Join(filterParts, " AND ")
}

type Server struct {
	RuntimeVersion        string
	AuthClient            *auth.Client
	SecretClient          *secretmanager.Client
	RegionalSecretClients map[string]*secretmanager.Client
	SmOpts                []option.ClientOption
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
	return handleMountEvent(ctx, s.SecretClient, gts, cfg, s.RegionalSecretClients, s.SmOpts)
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
func handleMountEvent(ctx context.Context, client *secretmanager.Client, creds credentials.PerRPCCredentials, cfg *config.MountConfig, regionalClients map[string]*secretmanager.Client, smOpts []option.ClientOption) (*v1alpha1.MountResponse, error) {
	results := make([]*secretmanagerpb.AccessSecretVersionResponse, len(cfg.Secrets))
	errs := make([]error, len(cfg.Secrets))

	// need to build a per-rpc call option based of the tokensource
	callAuth := gax.WithGRPCOptions(grpc.PerRPCCredentials(creds))

	// In parallel fetch all secrets needed for the mount
	wg := sync.WaitGroup{}
	out := &v1alpha1.MountResponse{}
	ovs := []*v1alpha1.ObjectVersion{}
	for i, secret := range cfg.Secrets {
		// Check if projectID, version, labels are provided
		if secret.ProjectID != "" && secret.Versions != "" && secret.Labels != nil {

			// Use the fetchSecrets function to fetch secrets
			secrets, err := fetchSecrets(ctx, secret.ProjectID, secret.Versions, secret.Labels, regionalClients, callAuth, smOpts)
			if err != nil {
				errs[i] = err
				continue
			}

			for _, s := range secrets {
				file := &v1alpha1.File{
					Path:     strings.ReplaceAll(s.Name[:strings.Index(s.Name, "/versions/")], "/", "_"), // Use the secret name as the file name
					Mode:     int32(cfg.Permissions),
					Contents: s.Payload.Data,
				}
				out.Files = append(out.Files, file)

				// Add object version to response
				// Assuming ovs is a slice of ObjectVersion
				ovs = append(ovs, &v1alpha1.ObjectVersion{
					Id:      secret.ProjectID + "/" + secret.Versions,
					Version: secret.Versions,
				})
			}
			klog.InfoS("Files created based on provided labels", "files", fmt.Sprintf("%d", len(ovs)), "skipped", fmt.Sprintf("%d", len(secrets)-len(ovs)))
			continue
		}
		loc, err := locationFromSecretResource(secret.ResourceName)
		if err != nil {
			errs[i] = err
			continue
		}

		if len(loc) > locationLengthLimit {
			errs[i] = fmt.Errorf("invalid location string, please check the location")
			continue
		}
		var secretClient *secretmanager.Client
		if loc == "" {
			secretClient = client
		} else {
			if _, ok := regionalClients[loc]; !ok {
				ep := option.WithEndpoint(fmt.Sprintf("secretmanager.%s.rep.googleapis.com:443", loc))
				regionalClient, err := secretmanager.NewClient(ctx, append(smOpts, ep)...)
				if err != nil {
					errs[i] = err
					continue
				}
				regionalClients[loc] = regionalClient
			}
			secretClient = regionalClients[loc]
		}
		wg.Add(1)
		i, secret := i, secret
		go func() {
			defer wg.Done()
			req := &secretmanagerpb.AccessSecretVersionRequest{
				Name: secret.ResourceName,
			}
			smMetricRecorder := csrmetrics.OutboundRPCStartRecorder("secretmanager_access_secret_version_requests")

			resp, err := secretClient.AccessSecretVersion(ctx, req, callAuth)
			if err != nil {
				if e, ok := status.FromError(err); ok {
					smMetricRecorder(csrmetrics.OutboundRPCStatus(e.Code().String()))
				}
			} else {
				smMetricRecorder(csrmetrics.OutboundRPCStatusOK)
			}
			results[i] = resp
			errs[i] = err
		}()
	}
	wg.Wait()

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

	// Add secrets to response.
	for i, secret := range cfg.Secrets {
		if secret.ProjectID == "" && secret.Versions == "" && len(secret.Labels) == 0 {

			if cfg.Permissions > math.MaxInt32 {
				return nil, fmt.Errorf("invalid file permission %d", cfg.Permissions)
			}
			// #nosec G115 Checking limit
			mode := int32(cfg.Permissions)
			if secret.Mode != nil {
				mode = *secret.Mode
			}

			result := results[i]
			out.Files = append(out.Files, &v1alpha1.File{
				Path:     secret.PathString(),
				Mode:     mode,
				Contents: result.Payload.Data,
			})
			klog.V(5).InfoS("added secret to response", "resource_name", secret.ResourceName, "file_name", secret.FileName, "pod", klog.ObjectRef{Namespace: cfg.PodInfo.Namespace, Name: cfg.PodInfo.Name})

			ovs = append(ovs, &v1alpha1.ObjectVersion{
				Id:      secret.ResourceName,
				Version: result.GetName(),
			})
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

// locationFromSecretResource returns location from the secret resource if the resource is in format "projects/<project_id>/locations/<location_id>/..."
// returns "" for global secret resource.
func locationFromSecretResource(resource string) (string, error) {
	globalSecretRegexp := regexp.MustCompile(globalSecretRegex)
	if m := globalSecretRegexp.FindStringSubmatch(resource); m != nil {
		return "", nil
	}
	regionalSecretRegexp := regexp.MustCompile(regionalSecretRegex)
	if m := regionalSecretRegexp.FindStringSubmatch(resource); m != nil {
		return m[2], nil
	}
	return "", status.Errorf(codes.InvalidArgument, "Invalid secret resource name: %s", resource)
}
