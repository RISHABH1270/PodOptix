{{/*
Chart naming helpers — used across all templates to keep resource names consistent.
*/}}

{{/* Base name — the release name */}}
{{- define "podoptix.name" -}}
{{- .Chart.Name -}}
{{- end -}}

{{/* Fullname — release-scoped, safe for cluster-unique names */}}
{{- define "podoptix.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Common labels applied to every resource */}}
{{- define "podoptix.labels" -}}
app.kubernetes.io/name: {{ include "podoptix.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/* Selector labels — subset of common labels used for pod selection */}}
{{- define "podoptix.selectorLabels" -}}
app.kubernetes.io/name: {{ include "podoptix.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* Per-component names — keep them short and predictable */}}
{{- define "podoptix.postgresName" -}}{{ include "podoptix.fullname" . }}-postgres{{- end -}}
{{- define "podoptix.redisName"    -}}{{ include "podoptix.fullname" . }}-redis{{- end -}}
{{- define "podoptix.secretName"   -}}{{ include "podoptix.fullname" . }}-secrets{{- end -}}
