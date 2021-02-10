# Debugging

## Events

If a pod fails to startup inspecting the pod events is a quick way to diagnose
issues:

```cli
$ kubectl describe pod mypod
Name:         mypod
Namespace:    default
...
Events:
  Type     Reason       Age                  From                                               Message
  ----     ------       ----                 ----                                               -------
  Normal   Scheduled    2m35s                default-scheduler                                  Successfully assigned default/mypod to gke-cluster-a-default-pool-3e0d8eb7-d2qk
  Warning  FailedMount  32s                  kubelet, gke-cluster-a-default-pool-3e0d8eb7-d2qk  Unable to attach or mount volumes: unmounted volumes=[mysecret], unattached volumes=[mysecret mypodserviceaccount-token-kwtrk]: timed out waiting for the condition
  Warning  FailedMount  24s (x9 over 2m35s)  kubelet, gke-cluster-a-default-pool-3e0d8eb7-d2qk  MountVolume.SetUp failed for volume "mysecret" : kubernetes.io/csi: mounter.SetupAt failed: rpc error: code = Unknown desc = failed to mount secrets store objects for pod default/mypod, err: rpc error: code = Internal desc = rpc error: code = NotFound desc = Secret Version [projects/REDACTED/secrets/testsecret/versions/100] not found.
```

The `FailedMount` event message ends with

> `rpc error: code = NotFound desc = Secret Version [projects/REDACTED/secrets/testsecret/versions/100] not found`

indicating that the referenced SecretVersion does not exist. In this case I
attempted to access version 100 but the Secret only had 3 versions.

It is also possible to get and filter events without the extra information
provided by `kubectl describe pod`:

```cli
kubectl get event --namespace=default --field-selector involvedObject.name=mypod
```

For further debugging you may need to find the `csi-secrets-store` driver or
`csi-secrets-store-provider-gcp` plugin pods that are involved in starting your 
pod. Find out what node the pod is scheduled on by passing `-o wide` and
looking for `NODE`:

```cli
$ kubectl get pods -o wide
NAME    READY   STATUS              RESTARTS   AGE   IP       NODE                                       NOMINATED NODE   READINESS GATES
mypod   0/1     ContainerCreating   0          7s    <none>   gke-cluster-a-default-pool-3e0d8eb7-d2qk   <none>           <none>
```

Then look for the `csi-secrets-store` and `csi-secrets-store-provider-gcp` pod
names on the same node:

```cli
$ kubectl get pods --all-namespaces --field-selector spec.nodeName=gke-cluster-a-default-pool-3e0d8eb7-d2qk
NAMESPACE     NAME                                                        READY   STATUS              RESTARTS   AGE
default       mypod                                                       0/1     ContainerCreating   0          2m11s
kube-system   csi-secrets-store-provider-gcp-n9c5f                        1/1     Running             0          23h
kube-system   csi-secrets-store-t9www                                     3/3     Running             0          4d3h
...
```

## Logs

The plugin uses the `klog` package to write logs to stderr. These are written as
`json` so that they can be properly parsed by Cloud Logging.

The following Cloud Logging filter will find logs from all
`csi-secrets-store-provider-gcp` pods:

```text
resource.labels.project_id="<project>"
resource.labels.location="<cluster location>"
resource.labels.cluster_name="<cluster name>"
resource.labels.namespace_name="kube-system"
labels.k8s-pod/app="csi-secrets-store-provider-gcp"
```

If you are having trouble with a single pod, you can expand the filter to cover
both the driver, plugin, and the pod pod:

```text
labels.k8s-pod/app=("csi-secrets-store-provider-gcp" OR "cloud-secrets-store")
jsonPayload.pod="default/mypod"
```

`kubectl` will also show you logs for a specific driver/plugin pod:

```cli
kubectl logs csi-secrets-store-provider-gcp-4jljs --namespace=kube-system
```

To increase verbosity of logs edit `deploy/provider-gcp-plugin.yaml` to set
`-v=5`:

```yaml
containers:
- name: provider
    ...
    args:
    - "-v=5"
```

## Metrics

View container status metrics on GKE by navigating to the 
`csi-secrets-store-provider-gcp` DeamonSet details page (`Workloads` then remove
the `Is system object` filter).

If the CPU or memory usage graphs appears to exceed limits then you may need to
tune `deploy/provider-gcp-plugin.yaml` and request more resources for the pods:

```yaml
...
resources:
requests:
    cpu: 50m
    memory: 100Mi
limits:
    cpu: 50m
    memory: 100Mi
...
```

Prometheus metrics are served from port 8095, but this port is not exposed
outside the pod by default. Use `kubectl port-forward` to access the
metrics over localhost:

```cli
kubectl port-forward csi-secrets-store-provider-gcp-vmqct --namespace=kube-system 8095:8095
curl localhost:8095/metrics
```

## pprof

Starting the plugin with `-enable-pprof=true` will enable a debug http endpoint
at `-debug_addr`.  Accessing this will also require `port-forward`:

```cli
kubectl port-forward csi-secrets-store-provider-gcp-vmqct --namespace=kube-system 6060:6060
curl localhost:6060/debug/pprof
```

## Objects

View `SecretProviderClass`s:

```cli
kubectl get secretproviderclass
```

View `SecretProviderClassPodStatuses`:

```cli
kubectl get secretproviderclasspodstatuses
```

This can be helpful to see the status of rotations or syncing to K8S secrets.
