# Authentication

This page documents the different ways that authentication can be configured for
`secrets-store-csi-driver-provider-gcp`.

## `pod-adc` - Pod Workload Identity (default)

The identity of the pod the secrets are mounted onto.

When the GCP provider receives a `Mount` request it obtains a Kubernetes
Service Account token for the associated pod. This token is then exchanged for
a Google authentication token using the Kubernetes Service Account [Workload
Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
annotations.

## `provider-adc` - GCP Provider Identity

In the `SecretProviderClass` you can set

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: app-secrets
spec:
  provider: gcp
  parameters:
    auth: provider-adc
    secrets: |
      ...
```

and the GCP provider will use its _own_
[Application Default Credentials](https://cloud.google.com/docs/authentication/production)
when calling the Secret Manager API.

This can be useful if you are using
[minikube and the GCP auth plugin](https://minikube.sigs.k8s.io/docs/handbook/addons/gcp-auth/)
as it will allow you to use your local `gcloud` identity to fetch secrets.

**NOTE:** This should not be used in production environments as it provides no
namespace isolation. All requests to the Secret Manager API will originate from
the same identity.

## `nodePublishSecretRef`

The Kubernetes implementation of CSI allows referencing a Kubernetes Secret for
volume mounts:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: secrets-store-inline-crd
spec:
  ...
  volumes:
    - name: secrets-store-inline
      csi:
        driver: secrets-store.csi.k8s.io
        readOnly: true
        volumeAttributes:
          secretProviderClass: "gcp"
        nodePublishSecretRef:
          name: secrets-store-creds
```

In this example the Kubernetes Secret `secrets-store-creds` will be passed along
to the GCP provider. If a `nodePublishSecretRef` is present then the drive will
use that identity. The Kubernetes Secret must have a key `key.json` with a value
of an exported GCP service account credential.

This may be useful in on-prem or multi-cloud environments, but in general it is
better to use
[Workload Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
instead.
