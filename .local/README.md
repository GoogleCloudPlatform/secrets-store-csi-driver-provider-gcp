# Overview
This clones the workflow laid out in [secrets store csi driver for local debugging](https://github.com/kubernetes-sigs/secrets-store-csi-driver/tree/main/.local).

Please review the sibling flow with its prerequisites, and then you can pick and choose what is needed on this side.

> NOTE: Steps in this guide are not tested by CI/CD. This is just one of the way to locally debug the code and a good starting point.

The debug driver was used to help flesh out issues with the federation of the workload identity. You must have a pod
that is attempting to mount the secret driver to have the debug breakpoints hit.

Review [docs/fleet-wif-notes.md](../docs/fleet-wif-notes.md) and the example [mypod.yaml.tmpl](../examples/mypod.yaml.tmpl)
for more details about setting up a consuming pod.

## Creating a docker image
- Build docker image from [Dockerfile](Dockerfile):

```sh
docker build -t debug-driver -f .local/Dockerfile .
```

## Update the debug-driver.yaml
Update the following items in .local/debug-driver.yaml:

* In the `workload-id-config` update the `audience` to match the audience of the workload identity pool provider.
* In the `debug-driver` container, update `driver-volume` to utilize a path on your local machine that is configured to be 
mounted into the pod.
* In the `gcp-ksa` volume, update the `audience` to match the audience of the workload identity pool provider.

Deploy the debug-driver and the consuming pod. You can then hook up your IDE to the debug-driver container via delve.
