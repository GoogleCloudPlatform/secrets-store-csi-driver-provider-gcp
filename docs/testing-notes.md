# Testing Notes

## Build and deploy notes

Use [Google Cloud Build](https://cloud.google.com/run/docs/building/containers#building_using) and [Container Registry](https://cloud.google.com/container-registry/docs/quickstart) to build and host the plugin docker image.

```shell
$ export PROJECT_ID=<your gcp project>
$ gcloud config set project $PROJECT_ID
$ ./scripts/build.sh
...
```

Deploy the plugin as a DaemonSet to your cluster.

```shell
$ ./scripts/deploy.sh
...
```

## Load tests

Setup:

```shell
export PROJECT_ID=tmurphy-gcp-oss
export TEST_SECRET_ID=small
gcloud secrets add-iam-policy-binding $TEST_SECRET_ID \
  --member=serviceAccount:$PROJECT_ID.svc.id.goog[default/test-cluster-sa] \
  --role=roles/secretmanager.secretAccessor
```

Usage:

```shell
./scripts/load-test.sh single
kubectl scale --replicas=100 deployment/test-load-one-secret
kubectl scale --replicas=0 deployment/test-load-one-secret
```

Additional subcommands:

* `./script/load-test.sh single` - a deployment where the pod references a `SecretProviderClass` with only 1 secret.
* `./script/load-test.sh many` - a deployment where the pod references a `SecretProviderClass` with 50 secrets.
* `./script/load-test.sh seed` - creates 50 secrets for use with `many`

Metric of interest:

* time to scale up
* memory usage of provider pods
* memory usage of driver pods
* individual failures (kubectl get events)

Limits and Dependencies:

| RPC/Metric |  Limit | Description |
|------|---|---|
| **kube-apiserver** `get` `serviceaccounts` | [5.0 QPS / 10 Burst](https://pkg.go.dev/k8s.io/client-go@v0.22.1/rest#pkg-constants) | This is 1:1 with `Mount` requests when using the [`pod-adc`](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/blob/main/docs/authentication.md#pod-adc---pod-workload-identity-default) auth method. |
| **kube-apiserver** `create` `serviceaccounts/token` | [5.0 QPS / 10 Burst](https://pkg.go.dev/k8s.io/client-go@v0.22.1/rest#pkg-constants) | This is 1:1 with `Mount` requests when using the [`pod-adc`](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/blob/main/docs/authentication.md#pod-adc---pod-workload-identity-default) auth method. In the future the token will be provided by the `kubelet` and/or driver process as part of the `Mount` requests when the driver adopts the `CSIServiceAccountToken` feature. |
| **GCP IAM** `GenerateAccessToken` | [60k QPM Quota](https://cloud.google.com/iam/quotas) |   |
| **GCP SM** `AccessSecretVersion` | [90k QPM Quota](https://cloud.google.com/secret-manager/quotas#request-rate-quotas) |   |
| `csi-secrets-store-provider-gcp` pod memory | 100MiB | The provider process. Must be high enough to fit all concurrent requested secrets in memory. |
| `csi-secrets-store` pod memory | 100MiB | The driver process. Must be high enough to fit all concurrent requested secrets in memory. |
