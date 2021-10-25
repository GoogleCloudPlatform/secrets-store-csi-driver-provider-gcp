# Testing Infrastructure

This folder codifies the test infrastructure used to execute end-to-end integration tests. Use these instruction to perform one-time provisioning of a GKE "management cluster".

E2E tests are executed in the management cluster and create/teardown separate test clusters for testing. See [test/e2e](test/e2e/README.md) for instructions on running the tests.

## Manual/Local Testing

```sh
$ ./scripts/build.sh
<redacted>
$ ./scripts/deploy.sh
<redacted>
```

## Management Cluster

Cluster created with:

```sh
gcloud container --project "secretmanager-csi-build" clusters create "test-mgmt-cluster" \
  --zone "us-central1-c" \
  --release-channel "regular" \
  --num-nodes "1" \
  --machine-type "e2-standard-4" \
  --disk-size "100" \
  --enable-ip-alias \
  --no-enable-master-authorized-networks \
  --addons ConfigConnector \
  --enable-autorepair \
  --workload-pool "secretmanager-csi-build.svc.id.goog" \
  --node-locations "us-central1-c" \
  --enable-stackdriver-kubernetes
```

Cluster configured with [config connector](https://cloud.google.com/config-connector/docs/how-to/install-upgrade-uninstall) and manually setup:

```sh
gcloud iam service-accounts create cnrm-system
gcloud iam service-accounts add-iam-policy-binding \
cnrm-system@secretmanager-csi-build.iam.gserviceaccount.com \
    --member="serviceAccount:secretmanager-csi-build.svc.id.goog[cnrm-system/cnrm-controller-manager]" \
    --role="roles/iam.workloadIdentityUser"
kubectl apply -f test/infra/managed/bootstrap/connector.yaml
kubectl apply -f test/infra/managed/namespaces/secretmanager-csi-build/namespace.yaml
kubectl apply --recursive -f ./test/infra/managed/
```

`test-mgmt-cluster` has a namespace `cnrm-system` with k8s service account `cnrm-controller-manager` that will actuate
all changes. This is tied to the `cnrm-system@secretmanager-csi-build.iam.gserviceaccount.com` GCP identity which has
wide privileges in the project.

## Test Workflow

```
Github Action   -> Cloud Build -> Driver docker image
                -> Cloud Build -> e2e test image
                -> Kubernetes
                                -> E2E Test
                                -> CNRM - New Cluster
                                -> CNRM - Secrets
                                            New Cluster
                                                Driver
                                                Test Worklows
```

## Service Accounts

* cnrm-system@secretmanager-csi-build.iam.gserviceaccount.com
  * Root in project, YAML for managing cluster resources

* gh-e2e-runner@secretmanager-csi-build.iam.gserviceaccount.com
  * Build e2e test images
  * Submit e2e yaml to `test-mgmt-cluster`

* e2e-test-sa@secretmanager-csi-build.iam.gserviceaccount.com
  * Service account that the e2e test container runs as
  * Writes yaml to cluster to create the test cluster and install CSI driver
  * Manages secrets for the integration test

* `secretmanager-csi-build.svc.id.goog[default/test-cluster-sa]`
  * Workload Identity used in number of e2e test (ephemeral test clusters)

* k8s-csi-test@secretmanager-csi-build.iam.gserviceaccount.com
  * The Identity for `secrets-store-csi-driver-e2e-gcp` test cases in https://github.com/kubernetes/test-infra
  * The workload identity of prow used in driver test cases

## Mgmt Cluster Configuration

To add or update resources make the change then run:

```sh
# switch to correct cluster
kubectl apply --recursive -f ./test/infra/managed/
```

Note: to delete resources you will need to delete the K8s resource, removing the
yaml from the repo will not delete the resource.

```sh
kubectl delete <resource>
```
