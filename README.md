# Google Secret Manager Provider for Secret Store CSI Driver

**WARNING:** This project is in active development and not suitable for
production use.

[Google Secret Manager](https://cloud.google.com/secret-manager/) provider for
the [Secret Store CSI
Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver). Allows you
to access secrets stored in Secret Manager as files mounted in Kubernetes pods.

## Build and deploy notes

* Create a new GKE cluster with K8S 1.16+
* Install [Secret Store CSI Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver) to the cluster.
```shell
$ kubectl apply -f deploy/rbac-secretproviderclass.yaml
$ kubectl apply -f deploy/csidriver.yaml
$ kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
$ kubectl apply -f deploy/secrets-store-csi-driver.yaml
```
* Use [Google Cloud Build](https://cloud.google.com/run/docs/building/containers#building_using) and [Container Registry](https://cloud.google.com/container-registry/docs/quickstart) to build and host the plugin docker image.
```shell
$ export PROJECT_ID=<your gcp project>
$ ./scripts/build.sh
```
* Deploy the plugin as a DaemonSet to your cluster.
```shell
$ ./scripts/deploy.sh
```
* Setup the workload identity service account
```shell
# Create a service account for workload identity
$ gcloud iam service-accounts create gke-workload

# Allow "default/mypod" to act as the new service account
$ gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:$PROJECT_ID.svc.id.goog[default/mypod]" \
    gke-workload@$PROJECT_ID.iam.gserviceaccount.com
```
* Create a secret that the workload identity service account can access
```shell
# Create a secret with 1 active version
$ echo "foo" > secret.data
$ gcloud secrets create test --replication-policy=automatic --data-file=secret.data
$ rm secret.data

# grant the new service account permission to access the secret
$ gcloud secrets add-iam-policy-binding test \
    --member=serviceAccount:gke-workload@$PROJECT_ID.iam.gserviceaccount.com \
    --role=roles/secretmanager.secretAccessor
```
* Try it out the example which attempts to mount the secret "test" in `$PROJECT_ID` to `/var/secrets/good1.txt` and `/var/secrets/good2.txt`
```shell
$ ./scripts/example.sh
$ kubectl exec -it mypod /bin/bash
root@mypod:/# ls /var/secrets
```

## Support

__This is not an officially supported Google product.__
