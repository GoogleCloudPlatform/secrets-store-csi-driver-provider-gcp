FROM golang:1.14 as build-env
WORKDIR /tmp/secrets-store-csi-driver-provider-gcp
COPY . ./
RUN CGO_ENABLED=0 go install github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/secrets-store-csi-driver-provider-gcp /bin/
ENTRYPOINT ["/bin/secrets-store-csi-driver-provider-gcp", "-daemonset"]
