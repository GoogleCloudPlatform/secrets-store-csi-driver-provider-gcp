{{/*
Expand the name of the chart.
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.labels" -}}
helm.sh/chart: {{ include "secrets-store-csi-driver-provider-gcp.chart" . }}
{{ include "secrets-store-csi-driver-provider-gcp.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.selectorLabels" -}}
app: {{ default "default" .Values.app }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.serviceAccountName" -}}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}

{{/*
Create the name of the daemon set to use
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.daemonSetName" -}}
{{- default "default" .Values.app }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.clusterRoleName" -}}
{{- .Chart.Name }}-role
{{- end }}

{{/*
Create the name of the cluster role binding to use
*/}}
{{- define "secrets-store-csi-driver-provider-gcp.clusterRoleBindingName" -}}
{{- .Chart.Name }}-rolebinding
{{- end }}
