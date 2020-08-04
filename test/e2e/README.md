# Configure E2E tests to execute in a cluster

```sh
sed "s/\$PROJECT_ID/${PROJECT_ID}/g" templates/pod.yaml.tmpl | kubectl apply -f -
```
