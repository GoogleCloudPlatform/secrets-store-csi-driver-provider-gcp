# Google Secret Manager Provider for Secret Store CSI Driver

**WARNING:** This project is in active development and not suitable for
production use.

[Google Secret Manager](https://cloud.google.com/secret-manager/) provider for
the [Secret Store CSI
Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver). Allows you
to access secrets stored in Secret Manager as files mounted in Kubernetes pods.

## Security Considerations

This plugin is built to ensure compatibility between Secret Manager and 
Kubernetes workloads that expect to load secrets from the filesystem. It also
enables syncing of those secrets to Kubernetes-native secrets for consumption
as environment variables.

When evaluating this plugin consider the following threats:

* When a secret is accessible on the filesystem, application vulnerabilities
  like [directory traversal][directory-traversal] attacks can become higher
  severity as the attacker may gain the ability read the secret material.
* When a secret is consumed through environment variables, misconfigurations
  such as enabling a debug endpoints or including dependencies that log process
  environment details may leak secrets.
* When copying secret material to another data store (like Kubernetes Secrets),
  consider whether the access controls on that data store are sufficiently
  narrow in scope.

For these reasons, _when possible_ we recommend using the Secret Manager API
directly (using one of the provided [client libraries][client-libraries], or by
following the [REST][rest] or [GRPC][grpc] documentation).

[client-libraries]: https://cloud.google.com/secret-manager/docs/reference/libraries
[rest]: https://cloud.google.com/secret-manager/docs/reference/rest
[grpc]: https://cloud.google.com/secret-manager/docs/reference/rpc
[directory-traversal]: https://en.wikipedia.org/wiki/Directory_traversal_attack

## Install

* Create a new GKE cluster with K8S 1.16+
* Install [Secret Store CSI Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver) v0.0.13 or higher to the cluster.
```shell
$ kubectl apply -f deploy/rbac-secretproviderclass.yaml
$ kubectl apply -f deploy/csidriver.yaml
$ kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
$ kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
$ kubectl apply -f deploy/secrets-store-csi-driver.yaml
```
* Install the plugin DaemonSet & additional RoleBindings
```shell
$ kubectl apply -f deploy/workload-id-binding.yaml
$ kubectl apply -f deploy/provider-gcp-plugin.yaml
```

## Build and deploy notes

* Use [Google Cloud Build](https://cloud.google.com/run/docs/building/containers#building_using) and [Container Registry](https://cloud.google.com/container-registry/docs/quickstart) to build and host the plugin docker image.
```shell
$ export PROJECT_ID=<your gcp project>
$ gcloud config set project $PROJECT_ID
$ ./scripts/build.sh
```
* Deploy the plugin as a DaemonSet to your cluster.
```shell
$ ./scripts/deploy.sh
```

## Usage

* Ensure that workload identity is [enabled](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#enable_on_existing_cluster) in your GKE cluster.
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
* Try it out the example which attempts to mount the secret "test" in `$PROJECT_ID` to `/var/secrets/good1.txt` and `/var/secrets/good2.txt`
```shell
$ ./scripts/example.sh
$ kubectl exec -it mypod /bin/bash
root@mypod:/# ls /var/secrets
```

## Contributing

Please see the [contributing guidelines](docs/contributing.md). Pull requests
and issues will be triaged weekly.

## Support

__This is not an officially supported Google product.__
