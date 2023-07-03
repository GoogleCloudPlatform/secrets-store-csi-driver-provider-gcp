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

// Package auth includes obtains auth tokens for workload identity.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
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
	if cfg.AuthNodePublishSecret {
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
// in driver spec, the provider does not receive any tokens drom driver and generates
// its own token
func (c *Client) Token(ctx context.Context, cfg *config.MountConfig) (*oauth2.Token, error) {

	idPool, idProvider, err := c.gkeWorkloadIdentity(ctx, cfg)
	if err != nil {
		idPool, idProvider, err = c.fleetWorkloadIdentity(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}

	klog.V(5).InfoS("workload id configured", "pool", idPool, "provider", idProvider)

	// Get iam.gke.io/gcp-service-account annotation to see if the
	// identitybindingtoken token should be traded for a GCP SA (Service Account) token.
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

	// Obtain a serviceaccount token for the pod
	var SATokenVal string
	if cfg.PodInfo.ServiceAccountTokens != "" {
		SAToken, err := c.ExtractSAToken(cfg, idPool) // calling function to extract token received from driver
		if err != nil {
			return nil, fmt.Errorf("unable to fetch SA token from driver: %w", err)
		}
		SATokenVal = SAToken.Token
	} else {
		SAToken, err := c.GeneratePodSAToken(ctx, cfg, idPool) // if no token received, provider generates its own token
		if err != nil {
			return nil, fmt.Errorf("unable to fetch pod token: %w", err)
		}
		SATokenVal = SAToken.Token
	}

	// Trade the kubernetes token for an identitybindingtoken token.
	idBindToken, err := tradeIDBindToken(ctx, c.HTTPClient, SATokenVal, idPool, idProvider)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch identitybindingtoken: %w", err)
	}

	// If no `iam.gke.io/gcp-service-account` annotation is present the
	// identitybindingtoken will be used directly, allowing bindings on secrets
	// of the form "serviceAccount:<project>.svc.id.goog[<namespace>/<sa>]".
	if gcpSA == "" {
		return idBindToken, nil
	}

	gcpSAResp, err := c.IAMClient.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
		Name:  fmt.Sprintf("projects/-/serviceAccounts/%s", gcpSA),
		Scope: secretmanager.DefaultAuthScopes(),
	}, gax.WithGRPCOptions(grpc.PerRPCCredentials(oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(idBindToken)})))
	if err != nil {
		return nil, fmt.Errorf("unable to fetch gcp service account token: %w", err)
	}
	return &oauth2.Token{AccessToken: gcpSAResp.GetAccessToken()}, nil
}

func (c *Client) ExtractSAToken(cfg *config.MountConfig, idPool string) (*authenticationv1.TokenRequestStatus, error) {
	AudienceTokens := map[string]authenticationv1.TokenRequestStatus{}
	json.Unmarshal([]byte(cfg.PodInfo.ServiceAccountTokens), &AudienceTokens)
	for k, v := range AudienceTokens {
		if k == idPool { // Only returns the token if the audience is the workload identity. Other tokens cannot be used.
			return &v, nil
		}
	}
	return nil, fmt.Errorf("Unable to obtain token from driver")
}

func (c *Client) GeneratePodSAToken(ctx context.Context, cfg *config.MountConfig, idPool string) (*authenticationv1.TokenRequestStatus, error) {
	ttl := int64((15 * time.Minute).Seconds())
	resp, err := c.KubeClient.CoreV1().
		ServiceAccounts(cfg.PodInfo.Namespace).
		CreateToken(ctx, cfg.PodInfo.ServiceAccount,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: &ttl,
					Audiences:         []string{idPool},
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
	projectID, err := c.MetadataClient.ProjectID()
	if err != nil {
		return "", "", fmt.Errorf("unable to get project id: %w", err)
	}
	idPool := fmt.Sprintf("%s.svc.id.goog", projectID)

	clusterLocation, err := c.MetadataClient.InstanceAttributeValue("cluster-location")
	if err != nil {
		return "", "", fmt.Errorf("unable to determine cluster location: %w", err)
	}
	clusterName, err := c.MetadataClient.InstanceAttributeValue("cluster-name")
	if err != nil {
		return "", "", fmt.Errorf("unable to determine cluster name: %w", err)
	}
	idProvider := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s", projectID, clusterLocation, clusterName)

	return idPool, idProvider, nil
}

func (c *Client) fleetWorkloadIdentity(ctx context.Context, cfg *config.MountConfig) (string, string, error) {
	const envVar = "GOOGLE_APPLICATION_CREDENTIALS"
	var jsonData []byte
	var err error
	if filename := os.Getenv(envVar); filename != "" {
		jsonData, err = ioutil.ReadFile(filepath.Clean(filename))
		if err != nil {
			return "", "", fmt.Errorf("google: error getting credentials using %v environment variable: %v", envVar, err)
		}
	}

	// Parse jsonData as one of the other supported credentials files.
	var f credentialsFile
	if err := json.Unmarshal(jsonData, &f); err != nil {
		return "", "", err
	}

	if f.Type != externalAccountKey {
		return "", "", fmt.Errorf("google: unexpected credentials type: %v, expected: %v", f.Type, externalAccountKey)
	}

	split := strings.SplitN(f.Audience, ":", 3)
	if split == nil || len(split) < 3 {
		return "", "", fmt.Errorf("google: unexpected audience value: %v", f.Audience)
	}
	idPool := split[1]
	idProvider := split[2]

	return idPool, idProvider, nil
}

func tradeIDBindToken(ctx context.Context, client *http.Client, k8sToken, idPool, idProvider string) (*oauth2.Token, error) {
	body, err := json.Marshal(map[string]string{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token_type":   "urn:ietf:params:oauth:token-type:jwt",
		"requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
		"subject_token":        k8sToken,
		"audience":             fmt.Sprintf("identitynamespace:%s:%s", idPool, idProvider),
		"scope":                "https://www.googleapis.com/auth/cloud-platform",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://securetoken.googleapis.com/v1/identitybindingtoken", bytes.NewBuffer(body)) //	A Request represents an HTTP request received by a server or to be sent by a client. In our case it is to be sent by client.
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not get idbindtoken token, status: %v", resp.StatusCode)
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	idBindToken := &oauth2.Token{}
	if err := json.Unmarshal(respBody, idBindToken); err != nil {
		return nil, err
	}
	return idBindToken, nil
}
