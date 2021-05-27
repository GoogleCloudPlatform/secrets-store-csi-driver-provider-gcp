# End-to-End Tests

This directory contains E2E tests that can be run as a job on a cluster. See [test/infra](test/infra/README.md) for instructions on how to configure the cluster.

E2E tests rely on [Config Connector](https://cloud.google.com/config-connector/docs/overview) to setup and tear-down test GKE clusters and assume that it is available.

# Run E2E tests (Presubmit)

Execute E2E tests by running the presubmit script. From `secrets-store-csi-driver-provider-gcp` directory.

```SH
$ export GCP_PROVIDER_SHA=main

$ test/infra/prow/presubmit.sh

# (Optional) Manually inspect view job logs for debugging.
$ kubectl logs -n e2e-test -l job-name=e2e-test-job -f
```
