#!/bin/bash
#
# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit  # Exit with error on non-zero exit codes
set -o pipefail # Check the exit code of *all* commands in a pipeline
set -o nounset  # Error if accessing an unbound variable
set -x          # Print each command as it is run

export CLUSTER_NAME=test-mgmt-cluster
export PROJECT_ID=secretmanager-csi-build
export SECRET_STORE_VERSION=${SECRET_STORE_VERSION:-v1.0.0}
export GKE_VERSION=${GKE_VERSION:-STABLE}
export GCP_PROVIDER_SHA=${GITHUB_SHA:-main}
export USE_GKE_GCLOUD_AUTH_PLUGIN=True

# Build the driver image
gcloud builds submit --config scripts/cloudbuild-dev.yaml --substitutions=TAG_NAME=${GCP_PROVIDER_SHA} --project $PROJECT_ID

# Build test images for E2E testing
gcloud builds submit --config test/e2e/cloudbuild.yaml --substitutions=TAG_NAME=${GCP_PROVIDER_SHA} --project $PROJECT_ID test/e2e

export JOB_NAME="e2e-test-job-$(head /dev/urandom | base64 | tr -dc 'a-z' | head -c 8)"

# Start up E2E tests
gcloud container clusters get-credentials $CLUSTER_NAME --zone us-central1-c --project $PROJECT_ID
sed "s/\$GCP_PROVIDER_SHA/${GCP_PROVIDER_SHA}/g;s/\$PROJECT_ID/${PROJECT_ID}/g;s/\$JOB_NAME/${JOB_NAME}/g;s/\$SECRET_STORE_VERSION/${SECRET_STORE_VERSION}/g;s/\$GKE_VERSION/${GKE_VERSION}/g" \
    test/e2e/e2e-test-job.yaml.tmpl | kubectl apply -f -

# Wait until job start, then subscribe to job logs
# kubctl wait doesn't work if the resource doesnt exist yet, so poll for the pod
# https://github.com/kubernetes/kubernetes/issues/83242
until kubectl get pod -l job-name="${JOB_NAME}" -n e2e-test -o=jsonpath='{.items[0].metadata.name}' >/dev/null 2>&1; do
    echo "Waiting for pod"
    sleep 1
done

kubectl wait pod --for=condition=ready -l job-name="${JOB_NAME}" -n e2e-test --timeout 2m
kubectl logs -n e2e-test -l job-name="${JOB_NAME}" -f | sed "s/^/TEST: /" &

while true; do
    if kubectl wait --for=condition=complete "job/${JOB_NAME}" -n e2e-test --timeout 0 > /dev/null 2>&1; then
        echo "Job completed"
        kubectl delete job "${JOB_NAME}" -n e2e-test
        exit 0
    fi

    if kubectl wait --for=condition=failed "job/${JOB_NAME}" -n e2e-test --timeout 0 > /dev/null 2>&1; then
        echo "Job failed"
        kubectl delete job "${JOB_NAME}" -n e2e-test
        exit 1
    fi

    sleep 60
done
