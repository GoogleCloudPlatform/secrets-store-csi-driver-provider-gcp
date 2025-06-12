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

// Package auth includes obtains auth tokens for workload identity.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/csrmetrics"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/vars"
	"github.com/googleapis/gax-go/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const cloudScope = "https://www.googleapis.com/auth/cloud-platform"

type Client struct {
	KubeClient     *kubernetes.Clientset
	MetadataClient *metadata.Client
	IAMClient      *credentials.IamCredentialsClient
	HTTPClient     *http.Client
}

// JSON key file types.
const (
	externalAccountKey = "external_account"
)

// credentialsFile is the unmarshalled representation of a credentials file.
type credentialsFile struct {
	Type string `json:"type"`
	// External Account fields
	Audience string `json:"audience"`
}

// TokenSource returns the correct oauth2.TokenSource depending on the auth
// configuration of the MountConfig.
func (c *Client) TokenSource(ctx context.Context, cfg *config.MountConfig) (oauth2.TokenSource, error) {
	allowSecretRef, err := vars.AllowNodepublishSecretRef.GetBooleanValue()
	if err != nil {
		klog.ErrorS(err, "failed to get ALLOW_NODE_PUBLISH_SECRET flag")
		klog.Fatal("failed to get ALLOW_NODE_PUBLISH_SECRET flag")
	}
	if cfg.AuthNodePublishSecret && allowSecretRef {
		creds, err := google.CredentialsFromJSON(ctx, cfg.AuthKubeSecret, cloudScope)
		if err != nil {
			return nil, fmt.Errorf("unable to generate credentials from key.json: %w", err)
		}
		return creds.TokenSource, nil
	}

	if cfg.AuthProviderADC {
		return google.DefaultTokenSource(ctx, cloudScope)
	}

	if cfg.AuthPodADC {
		token, err := c.Token(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("unable to obtain workload identity auth: %v", err)
		}
		return oauth2.StaticTokenSource(token), nil
	}

	return nil, errors.New("mount configuration has no auth method configured")
}

// Token fetches a workload identity auth token for the pod for the MountConfig.
//
// This requires obtaining a ServiceAccount token from the K8S API for the pod,
// trading that token for an identitybindingtoken using the
// securetoken.googleapis.com API, and then trading that token for a GCP
// Service Account token using the iamcredentials.googleapis.com API.
//
// Caveats:
//
// None of the API calls are cached since the plugin binary is executed once per
// mount event. The tokens are to be used immediately so no refresh abilities are
// implemented - blocking Issue #14.
//
// This method requires additional K8S API permission for the CSI driver
// daemonset, including serviceaccounts/token create and serviceaccounts get.
// These permissions could break node isolation and a long term solution is
// tracked by Issue #13.
//
// Token sent by driver is extracted and used. However, if tokenRequests is not set
// in driver spec, the provider does not receive any tokens from driver and generates
// its own token. Token creation can be removed once driver implements the requiresRepublish.
func (c *Client) Token(ctx context.Context, cfg *config.MountConfig) (*oauth2.Token, error) {

	var audience string
	idPool, idProvider, err := c.gkeWorkloadIdentity(ctx, cfg)
	if err != nil {
		idPool, idProvider, audience, err = c.fleetWorkloadIdentity(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}
	if audience == "" {
		audience = fmt.Sprintf("identitynamespace:%s:%s", idPool, idProvider)
		klog.V(5).InfoS("workload id configured", "pool", idPool, "provider", idProvider)
	} else {
		klog.V(5).InfoS("workload federation pool audience", audience)
	}

	// Get iam.gke.io/gcp-service-account annotation to see if the
	// identitybindingtoken token should be traded for a GCP SA token.
	// See https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#creating_a_relationship_between_ksas_and_gsas
	saResp, err := c.KubeClient.
		CoreV1().
		ServiceAccounts(cfg.PodInfo.Namespace).
		Get(ctx, cfg.PodInfo.ServiceAccount, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch SA info: %w", err)
	}
	gcpSA := saResp.Annotations["iam.gke.io/gcp-service-account"]
	klog.V(5).InfoS("matched service account", "service_account", gcpSA)

	// Obtain a serviceaccount token for the pod.
	var saTokenVal string
	if cfg.PodInfo.ServiceAccountTokens != "" {
		saToken, err := c.extractSAToken(cfg, idPool, audience) // calling function to extract token received from driver.
		if err != nil {
			return nil, fmt.Errorf("unable to fetch SA token from driver: %w", err)
		}
		saTokenVal = saToken.Token
	} else {
		saToken, err := c.generatePodSAToken(ctx, cfg, idPool, audience) // if no token received, provider generates its own token.
		if err != nil {
			return nil, fmt.Errorf("unable to fetch pod token: %w", err)
		}
		saTokenVal = saToken.Token
	}

	// Trade the kubernetes token for an identitybindingtoken token.
	idBindToken, err := tradeIDBindToken(ctx, c.HTTPClient, saTokenVal, audience)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch identitybindingtoken: %w", err)
	}

	// If no `iam.gke.io/gcp-service-account` annotation is present the
	// identitybindingtoken will be used directly, allowing bindings on secrets
	// of the form "serviceAccount:<project>.svc.id.goog[<namespace>/<sa>]".
	if gcpSA == "" {
		return idBindToken, nil
	}

	req := &credentialspb.GenerateAccessTokenRequest{
		Name:  fmt.Sprintf("projects/-/serviceAccounts/%s", gcpSA),
		Scope: secretmanager.DefaultAuthScopes(),
	}

	if gcpSADelegates, ok := saResp.Annotations["iam.gke.io/gcp-service-account-delegates"]; ok {
		var delegates []string
		if err := json.Unmarshal([]byte(gcpSADelegates), &delegates); err != nil {
			return nil, fmt.Errorf("unable to parse delegates annotation on SA: %w", err)
		}

		klog.V(5).InfoS("matched service account delegates", "service_account_delegates", delegates)

		for _, delegate := range delegates {
			req.Delegates = append(req.Delegates, fmt.Sprintf("projects/-/serviceAccounts/%s", delegate))
		}
	}

	gcpSAResp, err := c.IAMClient.GenerateAccessToken(ctx, req, gax.WithGRPCOptions(grpc.PerRPCCredentials(oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(idBindToken)})))
	if err != nil {
		return nil, fmt.Errorf("unable to fetch gcp service account token: %w", err)
	}
	return &oauth2.Token{AccessToken: gcpSAResp.GetAccessToken()}, nil
}

func (c *Client) extractSAToken(cfg *config.MountConfig, idPool, audience string) (*authenticationv1.TokenRequestStatus, error) {
	audienceTokens := map[string]authenticationv1.TokenRequestStatus{}
	if err := json.Unmarshal([]byte(cfg.PodInfo.ServiceAccountTokens), &audienceTokens); err != nil {
		return nil, err
	}
	for k, v := range audienceTokens {
		if k == idPool || k == audience { // Only returns the token if the audience is the workload identity. Other tokens cannot be used.
			return &v, nil
		}
	}
	return nil, fmt.Errorf("no token has audience value of idPool")
}

func (c *Client) generatePodSAToken(ctx context.Context, cfg *config.MountConfig, idPool, audience string) (*authenticationv1.TokenRequestStatus, error) {
	ttl := int64((15 * time.Minute).Seconds())
	_audience := idPool
	if _audience == "" {
		_audience = audience
	}
	resp, err := c.KubeClient.CoreV1().
		ServiceAccounts(cfg.PodInfo.Namespace).
		CreateToken(ctx, cfg.PodInfo.ServiceAccount,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: &ttl,
					Audiences:         []string{_audience},
					BoundObjectRef: &authenticationv1.BoundObjectReference{
						Kind:       "Pod", // Pod and secret are the only valid types
						APIVersion: "v1",
						Name:       cfg.PodInfo.Name,
						UID:        cfg.PodInfo.UID,
					},
				},
			},
			v1.CreateOptions{},
		)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pod token: %w", err)
	}
	return &resp.Status, nil
}

func (c *Client) gkeWorkloadIdentity(ctx context.Context, cfg *config.MountConfig) (string, string, error) {
	// Determine Workload ID parameters from the GCE instance metadata.
	projectID, err := vars.Project.GetValue()
	if err != nil {
		return "", "", fmt.Errorf("unable to read project name from environment: %w", err)
	}
	if projectID == "" {
		projectID, err = c.MetadataClient.ProjectIDWithContext(ctx)
		if err != nil {
			return "", "", fmt.Errorf("unable to get project id: %w", err)
		}
	}
	idPool := fmt.Sprintf("%s.svc.id.goog", projectID)

	clusterLocation, err := vars.ClusterLocation.GetValue()
	if err != nil {
		return "", "", fmt.Errorf("unable to read cluster location from environment: %w", err)
	}
	if clusterLocation == "" {
		clusterLocation, err = c.MetadataClient.InstanceAttributeValueWithContext(ctx, "cluster-location")
		if err != nil {
			return "", "", fmt.Errorf("unable to determine cluster location: %w", err)
		}
	}

	clusterName, err := vars.ClusterName.GetValue()
	if err != nil {
		return "", "", fmt.Errorf("unable to read cluster name from environment: %w", err)
	}
	if clusterName == "" {
		clusterName, err = c.MetadataClient.InstanceAttributeValueWithContext(ctx, "cluster-name")
		if err != nil {
			return "", "", fmt.Errorf("unable to determine cluster name: %w", err)
		}
	}

	gkeWorkloadIdentityProviderEndpoint, err := vars.GkeWorkloadIdentityEndPoint.GetValue()
	if err != nil {
		return "", "", fmt.Errorf("unable to read GKE workload identity provider endpoint: %w", err)
	}
	idProvider := fmt.Sprintf("%s/projects/%s/locations/%s/clusters/%s", gkeWorkloadIdentityProviderEndpoint, projectID, clusterLocation, clusterName)

	return idPool, idProvider, nil
}

func (c *Client) fleetWorkloadIdentity(ctx context.Context, cfg *config.MountConfig) (string, string, string, error) {
	const envVar = "GOOGLE_APPLICATION_CREDENTIALS"
	var jsonData []byte
	var err error
	if filename := os.Getenv(envVar); filename != "" {
		jsonData, err = os.ReadFile(filepath.Clean(filename))
		if err != nil {
			return "", "", "", fmt.Errorf("google: error getting credentials using %v environment variable: %v", envVar, err)
		}
	}

	// Parse jsonData as one of the other supported credentials files.
	var f credentialsFile
	if err := json.Unmarshal(jsonData, &f); err != nil {
		return "", "", "", err
	}

	if f.Type != externalAccountKey {
		return "", "", "", fmt.Errorf("google: unexpected credentials type: %v, expected: %v", f.Type, externalAccountKey)
	}

	split := strings.SplitN(f.Audience, ":", 3)
	if len(split) < 3 {
		// If the audience is not in the expected format, return the audience as the audience since this is likely a federated pool.
		return "", "", f.Audience, nil
	}
	idPool := split[1]
	idProvider := split[2]

	return idPool, idProvider, "", nil
}

func tradeIDBindToken(ctx context.Context, client *http.Client, k8sToken, audience string) (*oauth2.Token, error) {
	body, err := json.Marshal(map[string]string{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token_type":   "urn:ietf:params:oauth:token-type:jwt",
		"requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"subject_token":        k8sToken,
		"audience":             audience,
		"scope":                "https://www.googleapis.com/auth/cloud-platform",
	})
	if err != nil {
		return nil, err
	}

	identityBindingTokenEndPoint, err := vars.IdentityBindingTokenEndPoint.GetValue()

	if err != nil {
		return nil, fmt.Errorf("unable to read identity binding token endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", identityBindingTokenEndPoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	gcpIamMetricRecorder := csrmetrics.OutboundRPCStartRecorder("gcp_iam_get_id_bind_token_requests")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	gcpIamMetricRecorder(csrmetrics.OutboundRPCStatus(strconv.Itoa(resp.StatusCode)))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get idbindtoken token, status: %v", resp.StatusCode)
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	idBindToken := &oauth2.Token{}
	if err := json.Unmarshal(respBody, idBindToken); err != nil {
		return nil, err
	}
	return idBindToken, nil
}
