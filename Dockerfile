FROM golang:1.24 AS build-env

ARG TARGETARCH
ARG VERSION=dev
ARG GOPROXY=https://proxy.golang.org

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=$TARGETARCH \
    GOPROXY=${GOPROXY}

WORKDIR /tmp/secrets-store-csi-driver-provider-gcp
COPY . ./
RUN go get -t ./...
RUN make licensessave
RUN go install \
    -trimpath \
    -ldflags "-s -w -extldflags '-static' -X 'main.version=${VERSION}'" \
    github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp

FROM gcr.io/distroless/static-debian12
COPY --from=build-env /tmp/secrets-store-csi-driver-provider-gcp/licenses /licenses
COPY --from=build-env /go/bin/secrets-store-csi-driver-provider-gcp /bin/
ENTRYPOINT ["/bin/secrets-store-csi-driver-provider-gcp"]
