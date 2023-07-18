# secrets-store-csi-driver-provider-gcp Changelog

All notable changes to secrets-store-csi-driver-provider-gcp will be documented in this file. This file is maintained by humans and is therefore subject to error.

## unreleased

## v1.2.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.2.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.2.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.2.0`

Digest: `sha256:b7dde5ed536b2c6500c9237e14f6851cf8a2ff6d7a72656c3741be38e2cddf4d`

See CHANGELOG.md for more details.

### Added

* Per-secret file permissions [#182](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/182).
* `arm64` multi-platform image [#193](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/193).
* Fleet Workload Identity (Anthos Bare Metal) Support [#190](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/190) (read more at [docs/fleet-wif-notes.md](docs/fleet-wif-notes.md)).

### Changed

* Many dependencies updated and now built with go 1.20.

## v1.1.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.1.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.1.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.1.0`

Digest: `sha256:f7fd197984e95f777557ba9f6daef6c578f49bcddd1080fba0fe8f2c19fffd84`

### Changed

* Remove default logging of request/responses. This is intended to make logs
  less verbose and more actionable.
  [#161](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/161)

## v1.0.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.0.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.0.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v1.0.0`

Digest: `sha256:cb491d4af4776ac352aac415676918fa7cd4ef1671e71c579175ef3e2820af09`

### Changed

* No code changes. This release is corresponds with the `v1.0.0` release of the
[secrets-store-csi-driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver/releases/tag/v1.0.0).
* The deploy yaml no longer includes the `/var/lib/kubelet/pods` HostPath. This
was needed when the plugin wrote the files to the pod filesystems but is not
used since `v.0.5.0` set `-write_secrets=false`.

## v0.6.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.6.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.6.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.6.0`

Digest: `sha256:2733764e6c008fd5d846f7e8a0795502acdc5c0997aac2effb66f39776386786`

### Added

* `-sm_connection_pool_size` and `-iam_connection_pool_size` flags added with default value of `5`. Added as part of an optimization to reuse the same Secret Manager client across all `Mount` operations. [#94](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/94)
* Secrets can now be written to nested paths. The `path` parameter is also added as an alias for `fileName` in the `SecretProviderClass`. [#125](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/125)

### Removed

* The `-write_secrets` flag has been removed. All writes to the pod filesystem will now be done by the CSI driver
  instead of this plugin. This requires `v0.0.21+` of the `secrets-store-csi-driver`. [#98](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/98)

### Changed

* Updated dependencies to bring in an updated Secret Manager client with better retry behavior for `RESOURCE_EXHAUSTED` errors. [#135](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/135)
* Updated build to use go1.17 [#137](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/137)

## v0.5.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.5.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.5.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.5.0`

Digest: `sha256:f2e84e7ae583ae048be54c8083fe6c2708116d540c5955b9ad732ac512d50dd4`

### Changed

* The `-write_secrets` flag defaults to `false`. This requires `v0.0.21+` of the `secrets-store-csi-driver`. [#98](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/98)
* `wrote secret` and `added secret to response` log messages moved to level 5 (viewable by setting `-v=5`). [https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/120](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/120)

### Added

* Specify `auth: provider-adc` in the SecretProviderClass to use application default credentials instead of the mount's pod's workload identity. [#101](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/101)

## v0.4.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.4.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.4.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.4.0`

Digest: `sha256:d4b3b361dbf41ae407532358ec89510a20fc15d6e9620fd27f281c1e8f6db864`

### Added

* `-write_secrets` flag. Set `-write_secrets=false` with `v0.0.21+` of the `secrets-store-csi-driver` to have the driver write the secrets instead of this provider. This will be made the default in `v0.5.0`. [#98](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/issues/98)

## v0.3.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.3.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.3.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.3.0`

Digest: `sha256:90eeaef1afcbac988fb9f5a96222dff91f79920e9fa1c0d4200688ebc2680622`

### Changed

* `AccessSecretVersion` is attempted on all secrets in a `SecretProviderClass` before returning any errors or writing any data to the filesystem [#83](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/83). This allows all errors to be reported together. `SecretProviderClass`s that attempt to load ~hundreds of secrets may encounter memory pressure issues.

### Added

* `klog` structured logging [#80](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/80)
* `grpc` mount request and response metadata logging [#85](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/85)
* Initial prometheus metrics collection [#85](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/85)
* `livenessProbe` [#85](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/85)
* [Debugging documentation](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/blob/v0.3.0/docs/debugging.md) [#85](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/85)
* Optional pprof debugging endpoint [#88](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/88)

## v0.2.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.2.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.2.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.2.0`

Digest: `sha256:214f7aec249aaf450106eddd4455221f84283e8df2751ef5c70b6b1a69e598a0`

### Fixed

* Cleanup unix domain socket [#69](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/69)

### Changed

* Validate filenames against regex `[-._a-zA-Z0-9]+` and max length of 253 [#74](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/pull/74)

## v0.1.0

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.1.0`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.1.0`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin:v0.1.0`

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

## Initial Release

Images:

* `asia-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin@sha256:e8b491a72eb3f3337005565470972f41c52a8de47fc5266d6bf3e2a94d88df26`
* `europe-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin@sha256:e8b491a72eb3f3337005565470972f41c52a8de47fc5266d6bf3e2a94d88df26`
* `us-docker.pkg.dev/secretmanager-csi/secrets-store-csi-driver-provider-gcp/plugin@sha256:e8b491a72eb3f3337005565470972f41c52a8de47fc5266d6bf3e2a94d88df26`

Digest: `sha256:625419e2104639f16b45a068c05a1da3d9bb9e714a3f3486b0fb11628580b7c8`

* Initial image built from [`8929e57`](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/tree/8929e57f988dc87840d13c35235f5889d11c8005)
