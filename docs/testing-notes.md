# Build and deploy notes

* Use [Google Cloud Build](https://cloud.google.com/run/docs/building/containers#building_using) and [Container Registry](https://cloud.google.com/container-registry/docs/quickstart) to build and host the plugin docker image.
```shell
$ export PROJECT_ID=<your gcp project>
$ gcloud config set project $PROJECT_ID
$ ./scripts/build.sh
```
* Deploy the plugin as a DaemonSet to your cluster.
```shell
$ ./scripts/deploy.sh
```
