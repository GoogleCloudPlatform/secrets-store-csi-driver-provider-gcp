FROM golang:1.14 as build-env
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# e2e test
WORKDIR /tmp/secrets-store-csi-driver-provider-gcp/test/e2e
COPY . ./
RUN go get -t ./...
RUN go test -c .

# kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl

# deploy scripts
# TODO: think about how we want to version these
RUN curl -LO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/v0.0.12/deploy/rbac-secretproviderclass.yaml
RUN curl -LO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/v0.0.12/deploy/csidriver.yaml
RUN curl -LO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/v0.0.12/deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
RUN curl -LO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/v0.0.12/deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
RUN curl -LO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/v0.0.12/deploy/secrets-store-csi-driver.yaml

RUN curl -LO https://raw.githubusercontent.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/main/deploy/provider-gcp-plugin.yaml
RUN curl -LO https://raw.githubusercontent.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/main/deploy/workload-id-binding.yaml

# Use Cloud SDK image to use gCloud in tests
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine

WORKDIR /test
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/e2e.test /bin/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/templates /test/templates
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/kubectl /bin/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/rbac-secretproviderclass.yaml /test/deploy/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/csidriver.yaml /test/deploy/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml /test/deploy/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml /test/deploy/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/secrets-store-csi-driver.yaml /test/deploy/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/workload-id-binding.yaml /test/deploy/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/provider-gcp-plugin.yaml /test/deploy/
ENTRYPOINT ["/bin/e2e.test"]
