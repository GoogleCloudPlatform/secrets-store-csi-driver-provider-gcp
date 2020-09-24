# secrets-store-csi-driver-provider-gcp Changelog

All notable changes to secrets-store-csi-driver-provider-gcp will be documented in this file. This file is maintained by humans and is therefore subject to error.

## [unreleased]

### Breaking

Removed the following resources:

* `ClusterRoleBinding`: `secretproviderclasses-workload-id-rolebinding`
* `ClusterRole`: `secretproviderclasses-workload-id-role`

These RBAC rules gave the CSI driver the necesssary permissions to perform
workload ID auth. The introduction of the grpc interface will have the plugin
`DaemonSet` perform these operations instead.

Driver now requires v0.0.14 of the CSI driver with:
`--grpc-supported-providers=gcp;` set.

### Added

* Set Usage Agent String [#31](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/31)
* `DEBUG` environment variable [#40](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/40)

### Changed

* Plugin no longer needs to GET pod information [#29](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/29)

## [sha:8929e57f988dc87840d13c35235f5889d11c8005](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/tree/8929e57f988dc87840d13c35235f5889d11c8005)

* Initial image.
