# secrets-store-csi-driver-provider-gcp Changelog

All notable changes to secrets-store-csi-driver-provider-gcp will be documented in this file. This file is maintained by humans and is therefore subject to error.

## v0.1.0

Images:
* asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.1.0
* europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.1.0
* us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.1.0

Digest: `sha256:625419e2104639f16b45a068c05a1da3d9bb9e714a3f3486b0fb11628580b7c8`

### Breaking

If you were using a previous version, note that the following resources have
been removed and should be deleted from your cluster:

* `ClusterRoleBinding`: `secretproviderclasses-workload-id-rolebinding`
* `ClusterRole`: `secretproviderclasses-workload-id-role`

These RBAC rules gave the CSI driver the necesssary permissions to perform
workload ID auth. The introduction of the grpc interface will have the plugin
`DaemonSet` perform these operations instead.

Driver now requires v0.0.14+ of the CSI driver with:
`--grpc-supported-providers=gcp;` set.

### Added

* Set Usage Agent String [#31](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/31)
* `DEBUG` environment variable [#40](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/40)
* Support for `nodePublishSecretRef` authentication [#58](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/58)
* Switched to a grpc interface between the driver and plugin
  [#47](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/47).
  This enables support for experimental driver features including periodic
  re-fetching of secrets.

### Changed

* Plugin no longer needs to GET pod information [#29](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/29)
* The default installed namespace changed from `default` to `kube-system`

## [sha:8929e57f988dc87840d13c35235f5889d11c8005](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/tree/8929e57f988dc87840d13c35235f5889d11c8005)

* Initial image.
