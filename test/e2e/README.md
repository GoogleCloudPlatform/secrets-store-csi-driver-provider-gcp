# End-to-End Tests

This directory contains E2E tests that can be run as a job on a cluster. See [test/infra](test/infra/README.md) for instructions on how to configure the cluster.

E2E tests rely on [Config Connector](https://cloud.google.com/config-connector/docs/overview) to setup and tear-down test GKE clusters and assume that it is available.

## Test Secret Mounter

E2E tests run a simple test binary (test secret mounter) that reads a secret from a predefined location on the filesystem and writes it to a k8s configmap.

# Build Docker Images

```sh
$ export PROJECT_ID=myprojectid
$ export SECRET_STORE_VERSION=v0.0.12
$ export GCP_PROVIDER_BRANCH=main
$ ./build.sh
```

# Run E2E tests
To run end-to-end tests on a specific branch (after building images):

```sh
$ sed "s/\$GCP_PROVIDER_BRANCH/${GCP_PROVIDER_BRANCH}/g;s/\$PROJECT_ID/${PROJECT_ID}/g" e2e-test-job.yaml.tmpl | kubectl apply -f -

# view job logs
$ kubectl logs -n e2e-test -l job-name=e2e-test-job -f
```
