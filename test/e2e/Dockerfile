FROM golang:1.14 as build-env
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# e2e test
WORKDIR /tmp/secrets-store-csi-driver-provider-gcp/test/e2e
COPY . ./
RUN go get -t ./...
RUN go test -c .

# Use Cloud SDK image to use gCloud in tests
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine

ARG SECRET_STORE_VERSION
ARG GCP_PROVIDER_SHA

COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/e2e.test /bin/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/templates /test/templates

# kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv kubectl /bin/

WORKDIR /test/deploy

# TODO: Change this production yaml when https://github.com/kubernetes-sigs/secrets-store-csi-driver/pull/351 makes it into the secrets-store-csi-driver release
# secrets-store-csi-driver
RUN curl -fLO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/manifest_staging/deploy/rbac-secretproviderclass.yaml
RUN curl -fLO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/manifest_staging/deploy/csidriver.yaml
RUN curl -fLO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/manifest_staging/deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
RUN curl -fLO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/manifest_staging/deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
RUN curl -fLO https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/manifest_staging/deploy/secrets-store-csi-driver.yaml

# secrets-store-csi-driver-provider-gcp
RUN curl -fLO https://raw.githubusercontent.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/$GCP_PROVIDER_SHA/deploy/provider-gcp-plugin.yaml

WORKDIR /test
ENTRYPOINT ["/bin/e2e.test"]
