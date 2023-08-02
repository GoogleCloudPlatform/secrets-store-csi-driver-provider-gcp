# Google Secret Manager Provider for Secret Store CSI Driver

[![e2e](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/actions/workflows/e2e.yml/badge.svg)](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/actions/workflows/e2e.yml)

[Google Secret Manager](https://cloud.google.com/secret-manager/) provider for
the [Secret Store CSI
Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver). Allows you
to access secrets stored in Secret Manager as files mounted in Kubernetes pods.

## Install

* Create a new GKE cluster with Workload Identity or enable
  [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#enable_on_existing_cluster)
  on an existing cluster.
* Install the
  [Secret Store CSI Driver](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html)
  v1.0.1 or higher to the cluster.
* Install the Google plugin DaemonSet & additional RoleBindings:

```shell
kubectl apply -f deploy/provider-gcp-plugin.yaml
# if you want to use helm
# helm upgrade --install secrets-store-csi-driver-provider-gcp charts/secrets-store-csi-driver-provider-gcp
```

NOTE: The driver's rotation and secret syncing functionality is still in Alpha and requires [additional installation
steps](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html#optional-values).

## Usage

The provider will use the workload identity of the pod that a secret is mounted
onto when authenticating to the Google Secret Manager API. For this to work the
workload identity of the pod must be configured and appropriate IAM bindings
must be applied.

* Setup the workload identity service account.

```shell
$ export PROJECT_ID=<your gcp project>
$ gcloud config set project $PROJECT_ID
# Create a service account for workload identity
$ gcloud iam service-accounts create gke-workload

# Allow "default/mypod" to act as the new service account
$ gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:$PROJECT_ID.svc.id.goog[default/mypodserviceaccount]" \
    gke-workload@$PROJECT_ID.iam.gserviceaccount.com
```

* Create a secret that the workload identity service account can access

```shell
# Create a secret with 1 active version
$ echo "foo" > secret.data
$ gcloud secrets create testsecret --replication-policy=automatic --data-file=secret.data
$ rm secret.data

# grant the new service account permission to access the secret
$ gcloud secrets add-iam-policy-binding testsecret \
    --member=serviceAccount:gke-workload@$PROJECT_ID.iam.gserviceaccount.com \
    --role=roles/secretmanager.secretAccessor
```

* Try it out the [example](./examples) which attempts to mount the secret "test" in `$PROJECT_ID` to `/var/secrets/good1.txt` and `/var/secrets/good2.txt`

```shell
$ ./scripts/example.sh
$ kubectl exec -it mypod /bin/bash
root@mypod:/# ls /var/secrets
```

## Security Considerations

This plugin is built to ensure compatibility between Secret Manager and
Kubernetes workloads that need to load secrets from the filesystem. It also
enables syncing of those secrets to Kubernetes-native secrets for consumption
as environment variables.

When evaluating this plugin consider the following threats:

* When a secret is accessible on the **filesystem**, application vulnerabilities
  like [directory traversal][directory-traversal] attacks can become higher
  severity as the attacker may gain the ability to read the secret material.
* When a secret is consumed through **environment variables**, misconfigurations
  such as enabling a debug endpoint or including dependencies that log process
  environment details may leak secrets.
* When **syncing** secret material to another data store (like Kubernetes
  Secrets), consider whether the access controls on that data store are
  sufficiently narrow in scope.

For these reasons, _when possible_ we recommend using the Secret Manager API
directly (using one of the provided [client libraries][client-libraries], or by
following the [REST][rest] or [GRPC][grpc] documentation).

[client-libraries]: https://cloud.google.com/secret-manager/docs/reference/libraries
[rest]: https://cloud.google.com/secret-manager/docs/reference/rest
[grpc]: https://cloud.google.com/secret-manager/docs/reference/rpc
[directory-traversal]: https://en.wikipedia.org/wiki/Directory_traversal_attack

## Contributing

Please see the [contributing guidelines](docs/contributing.md).

## Support

__This is not an officially supported Google product.__

For support
[please search open issues here](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues),
and if your issue isn't already represented please
[open a new one](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/new/choose).
Pull requests and issues will be triaged weekly.
