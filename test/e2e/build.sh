#!/bin/bash

set -o errexit  # Exit with error on non-zero exit codes
set -o pipefail # Check the exit code of *all* commands in a pipeline
set -o nounset  # Error if accessing an unbound variable
set -x          # Print each command as it is run

docker build . \
       --build-arg SECRET_STORE_VERSION=${SECRET_STORE_VERSION} \
       --build-arg GCP_PROVIDER_BRANCH=${GCP_PROVIDER_BRANCH} \
       -t gcr.io/${PROJECT_ID}/e2e-test:${GCP_PROVIDER_BRANCH}
docker build test-secret-mounter \
       -t gcr.io/${PROJECT_ID}/test-secret-mounter:${GCP_PROVIDER_BRANCH}

docker push gcr.io/${PROJECT_ID}/e2e-test:${GCP_PROVIDER_BRANCH}
docker push gcr.io/${PROJECT_ID}/test-secret-mounter:${GCP_PROVIDER_BRANCH}
