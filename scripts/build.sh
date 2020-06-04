#!/bin/bash

set -o errexit  # Exit with error on non-zero exit codes
set -o pipefail # Check the exit code of *all* commands in a pipeline
set -o nounset  # Error if accessing an unbound variable
set -x          # Print each command as it is run

gcloud builds submit --tag gcr.io/${PROJECT_ID}/secrets-store-csi-driver-provider-gcp
