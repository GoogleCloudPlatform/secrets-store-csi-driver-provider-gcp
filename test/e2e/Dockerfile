FROM golang:1.18 as build-env
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# e2e test
WORKDIR /tmp/secrets-store-csi-driver-provider-gcp/test/e2e
COPY . ./
RUN go get -t ./...
RUN go test -c .

# Use Cloud SDK image to use gCloud in tests
ARG INSTALL_COMPONENTS=gke-gcloud-auth-plugin
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:debian_component_based

COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/e2e.test /bin/
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/templates /test/templates
COPY enable-rotation.sh /bin/

WORKDIR /test
ENTRYPOINT ["/bin/e2e.test"]
