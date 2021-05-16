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
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/config"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

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
func Token(ctx context.Context, cfg *config.MountConfig, clientset *kubernetes.Clientset) (*oauth2.Token, error) {
	// Determine Workload ID parameters from the GCE instance metadata.
	idPool, err := fetchIDPool()
	if err != nil {
		return nil, fmt.Errorf("unable to determine idPool: %w", err)
	}

	idProvider, err := fetchIDProvider()
	if err != nil {
		return nil, fmt.Errorf("unable to determine idProvider: %w", err)
	}

	klog.V(5).InfoS("workload id configured", "pool", idPool, "provider", idProvider)

	// Get iam.gke.io/gcp-service-account annotation to see if the
	// identitybindingtoken token should be traded for a GCP SA token.
	// See https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#creating_a_relationship_between_ksas_and_gsas
	saResp, err := clientset.CoreV1().ServiceAccounts(cfg.PodInfo.Namespace).Get(ctx, cfg.PodInfo.ServiceAccount, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch SA info: %w", err)
	}
	gcpSA := saResp.Annotations["iam.gke.io/gcp-service-account"]
	klog.V(5).InfoS("matched service account", "service_account", gcpSA)

	// Request a serviceaccount token for the pod
	ttl := int64((15 * time.Minute).Seconds())
	resp, err := clientset.CoreV1().ServiceAccounts(cfg.PodInfo.Namespace).CreateToken(ctx, cfg.PodInfo.ServiceAccount, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: &ttl,
			Audiences:         []string{idPool},
			BoundObjectRef: &authenticationv1.BoundObjectReference{
				Kind:       "Pod",
				APIVersion: "v1",
				Name:       cfg.PodInfo.Name,
				UID:        cfg.PodInfo.UID,
	resp, err := clientset.CoreV1().
		ServiceAccounts(cfg.PodInfo.Namespace).
		CreateToken(ctx, cfg.PodInfo.ServiceAccount, &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &ttl,
				Audiences:         []string{idPool},
				BoundObjectRef: &authenticationv1.BoundObjectReference{
					Kind:       "Pod",
					APIVersion: "v1",
					Name:       cfg.PodInfo.Name,
					UID:        cfg.PodInfo.UID,
				},
			},
		}, v1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pod token: %w", err)
	}

	// Trade the kubernetes token for an identitybindingtoken token.
	idBindToken, err := tradeIDBindToken(ctx, resp.Status.Token, idPool, idProvider)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch identitybindingtoken: %w", err)
	}

	// If no `iam.gke.io/gcp-service-account` annotation is present the
	// identitybindingtoken will be used directly, allowing bindings on secrets
	// of the form "serviceAccount:<project>.svc.id.goog[<namespace>/<sa>]".
	if gcpSA == "" {
		return idBindToken, nil
	}

	gcpSAClient, err := credentials.NewIamCredentialsClient(ctx, option.WithTokenSource(oauth2.StaticTokenSource(idBindToken)))
	if err != nil {
		return nil, fmt.Errorf("unable to create credentials client: %w", err)
	}
	gcpSAResp, err := gcpSAClient.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
		Name:  fmt.Sprintf("projects/-/serviceAccounts/%s", gcpSA),
		Scope: secretmanager.DefaultAuthScopes(),
		// TODO: set expiration
	})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch gcp service account token: %w", err)
	}
	return &oauth2.Token{AccessToken: gcpSAResp.GetAccessToken()}, nil
}

func tradeIDBindToken(ctx context.Context, k8sToken, idPool, idProvider string) (*oauth2.Token, error) {
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

	req, err := http.NewRequest("POST", "https://securetoken.googleapis.com/v1/identitybindingtoken", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
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

func fetchIDPool() (string, error) {
	id, err := metadata.ProjectID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.svc.id.goog", id), nil
}

func fetchIDProvider() (string, error) {
	projectID, err := metadata.ProjectID()
	if err != nil {
		return "", err
	}

	clusterLocation, err := metadata.InstanceAttributeValue("cluster-location")
	if err != nil {
		return "", err
	}

	clusterName, err := metadata.InstanceAttributeValue("cluster-name")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s", projectID, clusterLocation, clusterName), nil
}
