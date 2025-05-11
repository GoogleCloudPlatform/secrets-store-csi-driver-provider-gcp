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
export LOCATION_ID=us-central1

# Build the driver image
gcloud builds submit --config scripts/cloudbuild-dev.yaml --substitutions=TAG_NAME=${GCP_PROVIDER_SHA} --project $PROJECT_ID --timeout 2400

# Build test images for E2E testing
gcloud builds submit --config test/e2e/cloudbuild.yaml --substitutions=TAG_NAME=${GCP_PROVIDER_SHA} --project $PROJECT_ID test/e2e

BASE_JOB_NAME_RANDOM_SUFFIX=$(head /dev/urandom | base64 | tr -dc 'a-z' | head -c 8)
export JOB_NAME_SM="e2e-test-job-${BASE_JOB_NAME_RANDOM_SUFFIX}-sm"
export JOB_NAME_PM="e2e-test-job-${BASE_JOB_NAME_RANDOM_SUFFIX}-pm"

# Start up E2E tests
gcloud container clusters get-credentials $CLUSTER_NAME --zone us-central1-c --project $PROJECT_ID

echo "Applying Secret Manager E2E test job: ${JOB_NAME_SM}"
sed "s/\$GCP_PROVIDER_SHA/${GCP_PROVIDER_SHA}/g; \
    s/\$PROJECT_ID/${PROJECT_ID}/g; \
    s/\$LOCATION_ID/${LOCATION_ID}/g; \
    s/\$JOB_NAME/${JOB_NAME_SM}/g; \
    s/\$SECRET_STORE_VERSION/${SECRET_STORE_VERSION}/g; \
    s/\$GKE_VERSION/${GKE_VERSION}/g; \
    s/\$TEST_SUITE_NAME/secretmanager/g" \
    test/e2e/e2e-test-job.yaml.tmpl | kubectl apply -f -

echo "Applying Parameter Manager E2E test job: ${JOB_NAME_PM}"
sed "s/\$GCP_PROVIDER_SHA/${GCP_PROVIDER_SHA}/g; \
    s/\$PROJECT_ID/${PROJECT_ID}/g; \
    s/\$LOCATION_ID/${LOCATION_ID}/g; \
    s/\$JOB_NAME/${JOB_NAME_PM}/g; \
    s/\$SECRET_STORE_VERSION/${SECRET_STORE_VERSION}/g; \
    s/\$GKE_VERSION/${GKE_VERSION}/g; \
    s/\$TEST_SUITE_NAME/parametermanager/g" \
    test/e2e/e2e-test-job.yaml.tmpl | kubectl apply -f -

# Function to wait for a job's pod, then tail logs
# Arguments: $1=job_name, $2=log_prefix
setup_job_watch() {
    local job_name="$1"
    local log_prefix="$2"
    echo "Waiting for pod for job ${job_name}..."
    until kubectl get pod -l job-name="${job_name}" -n e2e-test -o=jsonpath='{.items[0].metadata.name}' >/dev/null 2>&1; do
        echo "Still waiting for pod for ${job_name}..."
        sleep 5
    done
    local pod_name
    pod_name=$(kubectl get pod -l job-name="${job_name}" -n e2e-test -o=jsonpath='{.items[0].metadata.name}')
    echo "Pod ${pod_name} found for job ${job_name}."

    echo "Waiting for pod ${pod_name} (job ${job_name}) to be ready..."
    kubectl wait pod "${pod_name}" --for=condition=ready -n e2e-test --timeout 5m # Increased timeout

    echo "Tailing logs for job ${job_name} (pod ${pod_name})..."
    kubectl logs -n e2e-test "${pod_name}" -f | sed "s/^/${log_prefix}[${job_name}]: /" &
}

setup_job_watch "${JOB_NAME_SM}" "SM_TEST"
setup_job_watch "${JOB_NAME_PM}" "PM_TEST"

SM_JOB_STATUS=-1 # -1: running, 0: success, 1: failed
PM_JOB_STATUS=-1 # -1: running, 0: success, 1: failed

# Function to check job status
# Arguments: $1=job_name
check_job_status() {
    local job_name="$1"
    if kubectl wait --for=condition=complete "job/${job_name}" -n e2e-test --timeout 0s > /dev/null 2>&1; then
        echo "Job ${job_name} completed successfully."
        kubectl delete job "${job_name}" -n e2e-test --ignore-not-found=true
        return 0
    elif kubectl wait --for=condition=failed "job/${job_name}" -n e2e-test --timeout 0s > /dev/null 2>&1; then
        echo "Job ${job_name} failed."
        kubectl delete job "${job_name}" -n e2e-test --ignore-not-found=true
        return 1
    elif ! kubectl get job "${job_name}" -n e2e-test > /dev/null 2>&1; then
        # If job object is gone and we didn't mark it complete/failed yet, it's an issue.
        echo "Job ${job_name} no longer exists and was not marked complete/failed. Assuming failure."
        return 1
    fi
    return 255 # Still running or unknown
}

echo "Monitoring job statuses..."
while [ "${SM_JOB_STATUS}" -eq -1 ] || [ "${PM_JOB_STATUS}" -eq -1 ]; do
    if [ "${SM_JOB_STATUS}" -eq -1 ]; then
        check_job_status "${JOB_NAME_SM}"
        ret=$?
        if [ $ret -ne 255 ]; then
            SM_JOB_STATUS=$ret
        fi
    fi

    if [ "${PM_JOB_STATUS}" -eq -1 ]; then
        check_job_status "${JOB_NAME_PM}"
        ret=$?
        if [ $ret -ne 255 ]; then
            PM_JOB_STATUS=$ret
        fi
    fi

    # If both are not equal to -1, both jobs have stopped now.
    if [ "${SM_JOB_STATUS}" -ne -1 ] && [ "${PM_JOB_STATUS}" -ne -1 ]; then
        break
    fi
    sleep 30 # Poll interval
done

wait # Wait for background log processes to finish or be terminated

echo "Final Job Statuses -- SM: ${SM_JOB_STATUS}, PM: ${PM_JOB_STATUS}"

if [ "${SM_JOB_STATUS}" -eq 0 ] && [ "${PM_JOB_STATUS}" -eq 0 ]; then
    echo "All E2E test jobs completed successfully."
    exit 0
else
    echo "One or more E2E test jobs failed."
    exit 1
fi
