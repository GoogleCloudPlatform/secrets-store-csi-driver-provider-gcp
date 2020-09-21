# Unrelease

## Breaking Changes

Removed the following resources:
* `ClusterRoleBinding`: `secretproviderclasses-workload-id-rolebinding`
* `ClusterRole`: `secretproviderclasses-workload-id-role`

These RBAC rules gave the CSI driver the necesssary permissions to perform
workload ID auth. The introduction of the grpc interface will have the plugin
`DaemonSet` perform these operations instead.

Driver now requires v0.0.14 of the CSI driver with:
`--grpc-supported-providers=gcp;` set.
