# Fleet Workload Identity Authentication

This page contains example configuration to configure the `secrets-store-csi-driver-provider-gcp` provider 
with [Fleet Workload Identity](https://cloud.google.com/anthos/fleet-management/docs/use-workload-identity) 
authentication in environments configured for 
[Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation) 
outside of the Google Cloud.

## `external_account` Credentials

Instead of the Google Service Account key file, it is possible to pass a Fleet Workload Identity configuration
JSON file to the process that needs authenticating to the Google API from the Kubernetes cluster configured for 
the Workload Identity Federation. The `secrets-store-csi-driver-provider-gcp` provider pods are such processes
that need to authenticate to the Google Secret Manager API to provide access to the application secrets.

Such configuration file contains `external_account` type of credential and does not contain any secrets similar to the
Google Service Account key. The configuration should be passed via the `GOOGLE_APPLICATION_CREDENTIALS` 
environment variable, which requires the file name of the file containing the configuration on 
the pod's local file system.

A ConfigMap to host the contents of the configuration file for the `GOOGLE_APPLICATION_CREDENTIALS` environment variable 
of pods on Kubernetes clusters, such as Anthos on Bare Metal clusters, that require accessing Google Cloud API using 
[Fleet Workload Identity](https://cloud.google.com/anthos/fleet-management/docs/use-workload-identity) can be created 
like illustrated in the following snippet:

---
```yaml
cat <<EOF | kubectl apply -f -
kind: ConfigMap
apiVersion: v1
metadata:
  namespace: kube-system
  name: default-creds-config
data:
  config: |
    {
      "type": "external_account",
      "audience": "identitynamespace:$FLEET_PROJECT_ID.svc.id.goog:https://gkehub.googleapis.com/projects/$FLEET_PROJECT_ID/locations/global/memberships/cluster1",
      "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/GSA_NAME@GSA_PROJECT_ID.iam.gserviceaccount.com:generateAccessToken",
      "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
      "token_url": "https://sts.googleapis.com/v1/token",
      "credential_source": {
        "file": "/var/run/secrets/tokens/gcp-ksa/token"
      }
    }
EOF
```
---

You can [Download the configuration](https://cloud.google.com/iam/docs/workload-download-cred-and-grant-access#download-configuration)
and create a ConfigMap to host the contents of the configuration file for the `GOOGLE_APPLICATION_CREDENTIALS` 
environment variable of pods on Kubernetes clusters that require accessing Google Cloud API using 
[Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation) enabled by a
[workload identity pool provider](https://cloud.google.com/iam/docs/best-practices-for-using-workload-identity-federation#provider-audience). 
The downloaded file will be similar to the following snippet and adhere to [AIP-4117](https://google.aip.dev/auth/4117):

---
```yaml
cat <<EOF | kubectl apply -f -
kind: ConfigMap
apiVersion: v1
metadata:
  namespace: kube-system
  name: default-creds-config
data:
  config: |
    {
      "type": "external_account",
      "audience": "//iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID",
      "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
      "token_info_url": "https://sts.googleapis.com/v1/introspect",
      "token_url": "https://sts.googleapis.com/v1/token",
      "universe_domain": "googleapis.com",
      "credential_source": {
        "file": "/var/run/secrets/tokens/gcp-ksa/token",
        "format": {
          "type": "text"
        }
      }
    }
EOF
```
---

Please note, that the `service_account_impersonation_url` attribute in the snippet above is only necessary if you 
link a Google Service Account with the Kubernetes Service account using `iam.gke.io/gcp-service-account` annotation
and `roles/iam.workloadIdentityUser` IAM role. Otherwise, please omit the attribute in the configuration.

---
## Pass `GOOGLE_APPLICATION_CREDENTIALS`

Following snippet illustrates passing the ConfigMap with `external_account` credential to the 
`secrets-store-csi-driver-provider-gcp` provider pods that needs Fleet Workload Identity Authentication 
for accessing Google Secret Manager secrets using the `GOOGLE_APPLICATION_CREDENTIALS` environment variable.

---
```yaml
spec:
  ...
  template:
    ...
    spec:
    ...
      containers:
        - name: provider
          image: gcr.io/$PROJECT_ID/secrets-store-csi-driver-provider-gcp:$GCP_PROVIDER_SHA
      ...
          env:
        ...
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: /var/run/secrets/tokens/gcp-ksa/google-application-credentials.json
          volumeMounts:
        ...
            - mountPath: /var/run/secrets/tokens/gcp-ksa
              name: gcp-ksa
              readOnly: true
      ...
      volumes:
      ...
        - name: gcp-ksa
          projected:
            defaultMode: 420
            sources:
            - serviceAccountToken:
                audience: $FLEET_PROJECT_ID.svc.id.goog
                expirationSeconds: 172800
                path: token
            - configMap:
                items:
                - key: config
                  path: google-application-credentials.json
                name: default-creds-config
                optional: false
```
---
## Set `GAIA_TOKEN_EXCHANGE_ENDPOINT` and appropriate audience
If you are using [Workload Identity Federation with Kubernetes](https://cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes#kubernetes),
you need to set in the `csi-secrets-store-provider-gcp` pod configuration the `GAIA_TOKEN_EXCHANGE_ENDPOINT` environment
variable to use the Security Token Service API, `https://sts.googleapis.com/v1/token`.
---
```yaml
spec:
  ...
  template:
    ...
    spec:
    ...
      containers:
        - name: provider
          image: gcr.io/$PROJECT_ID/secrets-store-csi-driver-provider-gcp:$GCP_PROVIDER_SHA
      ...
          env:
        ...
            - name: GOOGLE_APPLICATION_CREDENTIALS
              value: /var/run/secrets/tokens/gcp-ksa/google-application-credentials.json
            - name: GAIA_TOKEN_EXCHANGE_ENDPOINT
              value: https://sts.googleapis.com/v1/token
          volumeMounts:
        ...
            - mountPath: /var/run/secrets/tokens/gcp-ksa
              name: gcp-ksa
              readOnly: true
      ...
      volumes:
      ...
        - name: gcp-ksa
          projected:
            defaultMode: 420
            sources:
            - serviceAccountToken:
                audience: //iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID
                expirationSeconds: 172800
                path: token
            - configMap:
                items:
                - key: config
                  path: google-application-credentials.json
                name: default-creds-config
                optional: false
```
