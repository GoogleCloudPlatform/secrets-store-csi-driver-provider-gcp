# Use Cloud SDK base image for gCloud
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine

# kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv kubectl /bin/

# Image is only used as an environment. Entrypoint is specified by the "command" field in the Prow job config.
# https://github.com/kubernetes/test-infra/blob/master/prow/pod-utilities.md#pod-utilities
