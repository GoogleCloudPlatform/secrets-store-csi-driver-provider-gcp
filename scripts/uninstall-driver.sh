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

# Uninstall plugin
kubectl delete ds csi-secrets-store-provider-gcp -n kube-system
kubectl delete clusterrolebinding secrets-store-csi-driver-provider-gcp-rolebinding
kubectl delete sa secrets-store-csi-driver-provider-gcp -n kube-system
kubectl delete clusterrole secrets-store-csi-driver-provider-gcp-role

# Uninstall CSI Driver
kubectl delete ds csi-secrets-store-provider -n kube-system

kubectl delete clusterrolebinding secretproviderclasses-rolebinding
kubectl delete clusterrole secretproviderclasses-role

kubectl delete clusterrolebinding secretprovidersyncing-rolebinding
kubectl delete clusterrole secretprovidersyncing-role

kubectl delete sa secrets-store-csi-driver -n kube-system

kubectl delete crd secretproviderclasses.secrets-store.csi.x-k8s.io
kubectl delete crd secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io
