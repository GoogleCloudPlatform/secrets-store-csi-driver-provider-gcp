#!/bin/bash
#
# Copyright 2025 Google LLC
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

sed "s/\$PROJECT_ID/${PROJECT_ID}/g;s/\$LOCATION_ID/${LOCATION_ID}/g" examples/app-parameters.yaml.tmpl | kubectl apply -f -
sed "s/\$PROJECT_ID/${PROJECT_ID}/g;s/app-secrets/app-parameters/g" examples/mypod.yaml.tmpl | kubectl apply -f -
