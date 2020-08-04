FROM golang:1.14 as build-env
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# e2e test
WORKDIR /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/test-secret-mounter
COPY . ./
# RUN go get -t ./...
RUN go install \
    -trimpath \
    -ldflags "-s -w -extldflags '-static'" \
        github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/test/e2e/test-secret-mounter

# kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl

FROM alpine
COPY --from=build-env /go/bin/test-secret-mounter /bin/test-secret-mounter
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/test/e2e/test-secret-mounter/kubectl /bin/
ENTRYPOINT ["/bin/test-secret-mounter"]
