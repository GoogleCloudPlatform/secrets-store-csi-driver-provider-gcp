## TODO: Add architecture diagram

# Overview

This folder codifies the test infrastructure used to execute end-to-end integration tests. Use these instruction to perform one-time provisioning of a GKE "management cluster".

E2E tests are executed in the management cluster and create/teardown separate test clusters for testing. See [test/e2e](test/e2e/README.md) for instructions on running the tests.

# Set up Config Connector via Anthos Config Management

Set up Anthos Config Management on the management cluster to declaratively manage objects.

Follow the [instructions](https://cloud.google.com/anthos-config-management/docs/how-to/installing) to install Anthos Config Management.

1. Create and connect to a `management-cluster` GKE cluster in the project

```sh
$ export PROJECT_ID=<test infra project id>
$ gcloud config set project ${PROJECT_ID}

# Create the management cluster with workload identity enabled
$ gcloud container clusters create management-cluster \
  --release-channel regular \
  --zone us-central1-c \
  --workload-pool=${PROJECT_ID}.svc.id.goog

# This documentation assumes a default kubeconfig that connects to `management-cluster`
$ gcloud container clusters get-credentials management-cluster --zone us-central1-c --project ${PROJECT_ID}
```

1. Install Anthos Config Management and Config Connector in the management cluster.

```sh
# Enable the Anthos, Resource Manager, Secret Manager APIs
$ gcloud services enable anthos.googleapis.com
$ gcloud services enable cloudresourcemanager.googleapis.com
$ gcloud services enable secretmanager.googleapis.com

# Install Config Management Operator
# Obtain from gs://config-management-release/released/latest/config-management-operator.yaml
$ kubectl apply -f configs/config-management-operator.yaml

# Create ConfigManagement CRD
$ kubectl apply -f configs/config-management.yaml

# Install KCC (ACM does not yet support Workload Identity with KCC)
$ kubectl apply -f configs/kcc-bundle/install-bundle-workload-identity/
```

Wait for `cnrm-controller-manager-0` in namespace `cnrm-system` to be running.

1. Create a service account for Config Connector to use (via Workload Identity) to manage GCP resources.

```sh
$ gcloud iam service-accounts create cnrm-system --project ${PROJECT_ID}

# Allow KCC K8S SA to use cnrm-system IAM SA via Workload identity
$ gcloud iam service-accounts add-iam-policy-binding \
 cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com \
 --member="serviceAccount:${PROJECT_ID}.svc.id.goog[cnrm-system/cnrm-controller-manager]" \
 --role="roles/iam.workloadIdentityUser"

# Grant Config Connector permissions the project

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/iam.securityAdmin"

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/iam.serviceAccountAdmin"

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/compute.instanceAdmin.v1"

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/container.admin"

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/iam.serviceAccountUser"
```

1. [Install](https://cloud.google.com/anthos-config-management/docs/how-to/nomos-command#installing) the `nomos` tool

1. Use `nomos` to verify that the Anthos Config Management installation succeeded

```sh
# PENDING or SYNCED status means that the cluster is configured properly
$ nomos status
```

1. View KCC logs to verify that installation succeeded

```sh
$ kubectl logs cnrm-controller-manager-0 -n cnrm-system -f
```

# Test changes to anthos-managed

To test changes, use the `nomos` command to generate a YAML to apply to the management cluster:

```sh
$ cd anthos-managed
$ nomos hydrate --flat
$ kubectl apply -f compiled
```

# Configure Prow

1. Export service account key for `prow-pod-utils` IAM service account and store it in a k8s secret in the `test-pod` namespace for Prow Pod Utilities to access.

```sh
$ gcloud iam service-accounts keys create "sa-key.json" --project="${PROJECT}" --iam-account="prow-pod-utils@${PROJECT_ID}.iam.gserviceaccount.com"

$ kubectl create secret generic "service-account" -n "test-pods" --from-file="service-account.json=sa-key.json"

$ rm sa-key.json
```

This service account is granted access to the Prow instance GCS bucket that stores execution logs. Prow pod utils wrap logs and use the service account to store them.
