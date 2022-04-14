FROM golang:1.18 as build-env

ARG VERSION=dev

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /tmp/secrets-store-csi-driver-provider-gcp
COPY . ./
RUN go get -t ./...
RUN make licensessave
RUN go install \
    -trimpath \
    -ldflags "-s -w -extldflags '-static' -X 'main.version=${VERSION}'" \
    github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp

FROM gcr.io/distroless/static-debian10
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/licenses /licenses
COPY --from=build-env /go/bin/secrets-store-csi-driver-provider-gcp /bin/
ENTRYPOINT ["/bin/secrets-store-csi-driver-provider-gcp"]
