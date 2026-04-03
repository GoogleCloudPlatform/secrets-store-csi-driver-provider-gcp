FROM golang:1.22-alpine

ENV CGO_ENABLED=0
ENV GOROOT=/usr/local/go
ENV GOPATH=${HOME}/go
ENV PATH=$PATH:${GOROOT}/bin

RUN apk update && apk add --no-cache \
    git && \
    go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /secrets-store-csi-driver-provider-gcp-codebase

COPY go.mod go.mod
RUN go mod download

EXPOSE 30123

# these dlv debug arguments replicate driver args from DaemonSet
ENTRYPOINT ["/go/bin/dlv", "--listen=:30123", "--accept-multiclient", "--headless=true", "--api-version=2", "debug", "./", "--", "-v", "5"]
