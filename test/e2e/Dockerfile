FROM golang:1.24 AS build-env
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# e2e test
WORKDIR /tmp/secrets-store-csi-driver-provider-gcp/test/e2e

# Copy go.mod and go.sum first to leverage Docker cache for dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go mod tidy && go get -t ./...
RUN go test -c -tags="secretmanager_e2e" -o e2e_sm.test . # Compile Secret Manager specific tests
RUN go test -c -tags="parametermanager_e2e" -o e2e_pm.test . # Compile Parameter Manager specific tests
RUN go test -c -tags="all_e2e" -o e2e_all.test . # For running all tests

# Use Cloud SDK image to use gCloud in tests
ARG INSTALL_COMPONENTS=gke-gcloud-auth-plugin
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:debian_component_based
ENV USE_GKE_GCLOUD_AUTH_PLUGIN=True

# Copy the compiled test binaries
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/e2e_sm.test /bin/e2e_sm.test
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/e2e_pm.test /bin/e2e_pm.test
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/e2e_all.test /bin/e2e_all.test

# Copy necessary test assets
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/templates /test/templates
COPY enable-rotation.sh /bin/


# Copy the run_tests.sh script from the build context (same directory as Dockerfile)
# into the /test directory in the image and make it executable.
COPY run_tests.sh /test/run_tests.sh
RUN chmod +x /test/run_tests.sh

WORKDIR /test
ENTRYPOINT ["./run_tests.sh"]
