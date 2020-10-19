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

# This script will install CSI driver with all features enabled into kube-system.

set -o errexit  # Exit with error on non-zero exit codes
set -o pipefail # Check the exit code of *all* commands in a pipeline
set -o nounset  # Error if accessing an unbound variable
set -x          # Print each command as it is run

SECRET_STORE_VERSION=v0.0.16
GCP_PROVIDER_SHA=v0.1.0

# -- CSI Driver Info --
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/deploy/csidriver.yaml

# -- CRD --
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml

# -- RBAC --
# namespace: default => namespace: kube-system
curl -s https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/deploy/rbac-secretproviderclass.yaml  2>&1 |
    sed "s/namespace: default/namespace: kube-system/g;" |
    kubectl apply -f -

curl -s https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/deploy/rbac-secretprovidersyncing.yaml  2>&1 |
    sed "s/namespace: default/namespace: kube-system/g;" |
    kubectl apply -f -

# -- DaemonSet --
# namespace: default => namespace: kube-system
# set "--grpcSupportedProviders=gcp;"
curl -s https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/$SECRET_STORE_VERSION/deploy/secrets-store-csi-driver.yaml  2>&1 |
    awk '/rotation-poll-interval/ && !x {print "            - \"--grpc-supported-providers=gcp;\""; x=1} 1' |
    sed "s/--enable-secret-rotation=false/--enable-secret-rotation=true/g;" |
    kubectl apply -n kube-system -f -

# -- GCP Plugin --
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/$GCP_PROVIDER_SHA/deploy/provider-gcp-plugin.yaml
