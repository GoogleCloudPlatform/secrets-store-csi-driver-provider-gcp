#!/bin/bash
#
# Copyright 2021 Google LLC
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

# Usage ./scripts/load-test.sh <COMMAND>
#
# See docs/testing-notes.md for full details
if [ "$1" == "many"  ]; then
    sed "s/\$PROJECT_ID/${PROJECT_ID}/g;s/\$TEST_SECRET_ID/${TEST_SECRET_ID}/g" test/e2e/templates/load-many-secrets.yaml.tmpl | kubectl apply -f -
elif [ "$1" == "single" ]; then
    sed "s/\$PROJECT_ID/${PROJECT_ID}/g;s/\$TEST_SECRET_ID/${TEST_SECRET_ID}/g" test/e2e/templates/load-one-secret.yaml.tmpl | kubectl apply -f -
elif [ "$1" == "seed" ]; then
    for i in {1..50}; do
        printf "s3cr3t" | gcloud secrets create ${TEST_SECRET_ID}-${i} --data-file=- || true
        gcloud secrets add-iam-policy-binding ${TEST_SECRET_ID}-${i} \
           --member=serviceAccount:$PROJECT_ID.svc.id.goog[default/test-cluster-sa] \
           --role=roles/secretmanager.secretAccessor
        # give the API a rest between creates
        sleep 1
    done
fi
