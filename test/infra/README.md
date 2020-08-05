## TODO: Add architecture diagram

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

# This documentation assumes a default kubeconfig that connects to `managment-cluster`
$ gcloud container clusters get-credentials management-cluster --zone us-central1-c --project ${PROJECT_ID}
```

1. Install Anthos Config Management and Config Connector in the management cluster.

```sh
# Enable the Anthos, Resource Manager, Secret Manager APIs
$ gcloud services enable anthos.googleapis.com
$ gcloud services enable cloudresourcemanager.googleapis.com
$ gcloud services enable secretmanager.googleapis.com

# Install Config Management Operator
$ kubectl apply -f configs/config-management-operator.yaml

# Create ConfigManagement CRD
$ kubectl apply -f configs/config-management.yaml
```

1. [Install](https://cloud.google.com/anthos-config-management/docs/how-to/nomos-command#installing) the `nomos` tool

1. Use `nomos` to verify that the installation succeeded

```sh
# PENDING or SYNCED status means that the cluster is configured properly
$ nomos status
```

1. Create a service account for Config Connector to use to manage GCP resources.
## TODO: Do this via Workload identity after b/154765441 is resolved

```sh
$ gcloud iam service-accounts create cnrm-system --project ${PROJECT_ID}

# Grant Config Connector permission on GKE and GCE and to use service accounts
$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/compute.instanceAdmin.v1"

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/container.admin"

$ gcloud projects add-iam-policy-binding ${PROJECT_ID} \
 --member "serviceAccount:cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" \
 --role "roles/iam.serviceAccountUser"

# Export service account key and store in k8s secret for ConfigConnector to use.
$ gcloud iam service-accounts keys create --iam-account "cnrm-system@${PROJECT_ID}.iam.gserviceaccount.com" ./key.json
$ kubectl create secret generic gcp-key --from-file ./key.json --namespace cnrm-system
$ rm key.json
```

Wait for `cnrm-controller-manager-0` in namespace `cnrm-system` to be running.
