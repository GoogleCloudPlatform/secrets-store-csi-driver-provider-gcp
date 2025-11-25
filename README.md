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

### Secret Manager

The provider will use the workload identity of the pod that a secret is mounted
onto when authenticating to the Google Secret Manager API. For this to work the
workload identity of the pod must be configured and appropriate IAM bindings
must be applied.

* Setup the workload identity service account.

```shell
$ export PROJECT_ID=<your gcp project>
$ gcloud config set project $PROJECT_ID
$ export PROJECT_NUMBER="$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')"
```

* Create a secret that the workload identity service account can access

```shell
# Create a secret with 1 active version
$ echo "foo" > secret.data
$ gcloud secrets create testsecret --replication-policy=automatic --data-file=secret.data
$ rm secret.data

# grant the new service account permission to access the secret
$ gcloud secrets add-iam-policy-binding testsecret \
    --role=roles/secretmanager.secretAccessor \
    --member=principal://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${PROJECT_ID}.svc.id.goog/subject/ns/default/sa/mypodserviceaccount
```

* Note: Regional secrets are also supported from v1.6.0, Please see [Regional Secret Documentation](https://cloud.google.com/secret-manager/regional-secrets/config-sm-rs).

* Try it out the [example](./examples) which attempts to mount the secret "test" in `$PROJECT_ID` to `/var/secrets/good1.txt` and `/var/secrets/good2.txt`

```shell
$ ./scripts/example.sh
$ kubectl exec -it mypod /bin/bash
root@mypod:/# ls /var/secrets
```


### Parameter Manager

From version 1.9.0, secrets-store-csi-driver-provider-gcp also supports mounting the parameter version from [Google Parameter Version](https://cloud.google.com/secret-manager/parameter-manager/docs/overview)

Parameter Manager is an extension to the Secret Manager service and provides a centralized storage for all configuration parameters related to your workload deployments.

* Setup the workload identity service account if not done already for secret manager example.

```shell
$ export PROJECT_ID=<your gcp project>
$ gcloud config set project $PROJECT_ID
$ export PROJECT_NUMBER="$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')"
```

* Create a parameter that the workload identity service account can access in the supported [location](https://cloud.google.com/secret-manager/docs/locations#parameter_manager_locations).

```shell
# set the  location
$ export LOCATION_ID=<location>
$ gcloud config set api_endpoint_overrides/parametermanager https://parametermanager.${LOCATION_ID}.rep.googleapis.com/
$ echo "server_port: 8080" > parameter.data
$  gcloud parametermanager parameters  create testparameter --location ${LOCATION_ID} --parameter-format=YAML --project ${PROJECT_ID}
$ gcloud parametermanager parameters versions create testversion --parameter=testparameter --location=${LOCATION_ID} --payload-data-from-file=parameter.data
$ rm parameter.data

# grant the new service account permission to access the secret
$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --role=roles/parametermanager.parameterAccessor \
    --member=principal://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${PROJECT_ID}.svc.id.goog/subject/ns/default/sa/mypodserviceaccount
```

* Try it out the [example](./examples) which attempts to mount the parameterversions

```shell
$ ./scripts/pm_example.sh
# wait for pod to be in running state
$ kubectl exec -it mypod -- cat /var/secrets/good1.txt
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

We close issues after 30 days if there's been no response or action taken.