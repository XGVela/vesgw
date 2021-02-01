# Copyright 2021 Mavenir
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
{{- define "nf.vendorId" -}}
{{- default "-" $.Values.nf.vendorId | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nf.xgvelaId" -}}
{{- default "-" $.Values.nf.xgvelaId | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nf.nfClass" -}}
{{- default "-" $.Values.nf.nfClass | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nf.nfType" -}}
{{- default "-" $.Values.nf.nfType | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nf.nfId" -}}
{{- default "-" $.Values.nf.nfId | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nf.version" -}}
{{- default "-" $.Values.nf.version | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "topogw.fqdn" -}}
{{- default "-" $.Values.global.xgvela.topogw_fqdn | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "nf.component_name" -}}
{{- $.Chart.Name | trunc 63 | trimSuffix "-" -}}
{{ end -}}

{{- define "nf.release_ns" -}}
{{- $.Release.Namespace | trunc 63 | trimSuffix "-" -}}
{{ end -}}

{{- define "nf.prefix_2_9" -}}
{{- printf "%s-%s-%s-%s-%s" (include "nf.vendorId" (dict "Values" $.Values) |toString) (include "nf.xgvelaId" (dict "Values" $.Values) |toString) (include "nf.nfClass" (dict "Values" $.Values) |toString) (include "nf.nfType" (dict "Values" $.Values) |toString) (include "nf.nfId" (dict "Values" $.Values) |toString) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "tmaasSpec_2_9" -}}
{{- $specOffset := .specOffset -}}
{{- printf "xgvela.com/tmaas: '{" | nindent (add $specOffset 0 | int) -}}
{{- printf "\"vendorId\": \"%s\"," (.nfVariables.vendorId | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "\"xgvelaId\": \"%s\"," (.nfVariables.xgvelaId | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "\"nfClass\": \"%s\"," (.nfVariables.nfType | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "\"nfType\": \"%s\"," (.nfVariables.nfName | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "\"nfId\": \"%s\"," (.nfVariables.nfId | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "\"nfServiceId\": \"%s\"," (.nfVariables.nfServiceId | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "\"nfServiceType\": \"%s\"" (.nfVariables.nfServiceType | toString) | nindent (add $specOffset 2 | int) -}}
{{- printf "}'" | nindent (add $specOffset 0 | int) -}}
{{- end -}}

{{- define "cnfTplHeader_2_9" -}}
{{- $defaultVariables := $.dot.Values.default -}}
{{- $nfVariables := (dict "nfName" (include "nf.nfType" (dict "Values" $.dot.Values) |toString) "nfType" (include "nf.nfClass" (dict "Values" $.dot.Values) |toString) "xgvelaId" (include "nf.xgvelaId" (dict "Values" $.dot.Values) |toString) "vendorId" (include "nf.vendorId" (dict "Values" $.dot.Values) |toString) "vendorId" (include "nf.vendorId" (dict "Values" $.dot.Values) |toString) "nfId" (include "nf.nfId" (dict "Values" $.dot.Values) |toString) "nfServiceId" (include "nf.component_name" (dict "Chart" $.dot.Chart) |toString) "nfServiceType" (include "nf.component_name" (dict "Chart" $.dot.Chart) |toString) "svcVersion" (include "nf.version" (dict "Values" $.dot.Values) |toString) "nfPrefix" (include "nf.prefix_2_9" (dict "Values" $.dot.Values) |toString) "svcname" (include "nf.component_name" (dict "Chart" $.dot.Chart) |toString) "component_name" (include "nf.component_name" (dict "Chart" $.dot.Chart) |toString) "topogwFQDN" (include "topogw.fqdn" (dict "Values" $.dot.Values) |toString) "defaultVariables" ($defaultVariables) "root" ($.dot) "create_meta_ns" "true") -}}
{{- if and ($nfVariables.root.Values.global) ($nfVariables.root.Values.global.xgvela) ($nfVariables.root.Values.global.xgvela.use_release_ns) -}}
{{- $_ := set $nfVariables "nfPrefix" (include "nf.release_ns" (dict "Release" $.dot.Release) |toString) -}}
{{- end -}}
{{- $_ := merge $.cnfHdr (dict "namespace" $nfVariables.nfPrefix) -}}
{{- $_ := merge $.cnfHdr (dict "specOffset" "0") -}}
{{- $_ := merge $.cnfHdr (dict "nfVariables" $nfVariables) -}}
{{- end -}}

{{- define "cnfTplMetadata_2_9" -}}
{{- include "metaSpec_2_9" (merge (dict "specOffset" (add $.setOffset 0 | int) "metaSpec" $.metadata) (dict "nfVariables" $.cnfHdr.nfVariables)) -}}
{{- end -}}

{{- define "eHdr" -}}
{{- $key := .key -}}
{{- $value := .value -}}
{{- $hdrOffset := .hdrOffset -}}
{{- if $value -}}
{{- printf "%s: " $key | nindent $hdrOffset -}}{{- $value -}}
{{- end -}}
{{- end -}}

{{- define "cHdr" -}}
{{- $key := .key -}}
{{- $value := .value -}}
{{- $hdrOffset := .hdrOffset -}}
{{- with $value -}}
{{- if $key -}}
{{- printf "%s: " $key  | nindent $hdrOffset -}}
{{- toYaml $value | trimSuffix "\n" | indent 2 | nindent $hdrOffset -}}
{{- else -}}
{{- toYaml $value | trimSuffix "\n" | indent 0 | nindent $hdrOffset -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "metaSpec_2_9" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "metadata" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- if .metaSpec -}}
{{- if .metaSpec.ext_name -}}
{{- include "eHdr" (dict "key" "name" "value" (printf "%s-%s" .nfVariables.component_name .metaSpec.ext_name) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- else -}}
{{- include "eHdr" (dict "key" "name" "value" (printf "%s" .nfVariables.component_name) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- end -}}
{{- else -}}
{{- include "eHdr" (dict "key" "name" "value" (printf "%s" .nfVariables.component_name) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- end -}}
{{- if eq (.nfVariables.create_meta_ns|toString) "true" -}}
{{- include "eHdr" (dict "key" "namespace" "value" (.nfVariables.nfPrefix) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- end -}}
{{- include "eHdr" (dict "key" "labels" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "microSvcName" "value" (.nfVariables.svcname) "hdrOffset" (add $specOffset 4 | int)) -}}
{{- include "eHdr" (dict "key" "xgvelaId" "value" (.nfVariables.xgvelaId) "hdrOffset" (add $specOffset 4 | int)) -}}
{{- if .metaSpec -}}
{{- include "cHdr" (dict "key" "" "value" .metaSpec.labels "hdrOffset" (add $specOffset 4 | int)) -}}
{{- end -}}
{{- include "eHdr" (dict "key" "annotations" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "tmaasSpec_2_9" (merge (dict "specOffset" (add $specOffset 4 | int)) (dict "nfVariables" .nfVariables)) -}}
{{- printf "svcVersion: \"%s\"" (.nfVariables.svcVersion | toString) | nindent (add $specOffset 4 | int) -}}
{{- printf "init: \"true\"" | nindent (add $specOffset 4 | int) -}}
{{- printf "topogw.fqdn: \"%s\"" (.nfVariables.topogwFQDN | toString) | nindent (add $specOffset 4 | int) -}}
{{- include "eHdr" (dict "key" "nwFnPrefix" "value" (.nfVariables.nfPrefix) "hdrOffset" (add $specOffset 4 | int) ) -}}
{{/*- include "eHdr" (dict "key" "svcVersion" "value" (.nfVariables.svcVersion | toString) "hdrOffset" (add $specOffset 4 | int) ) -*/}}
{{- if .metaSpec -}}
{{- include "cHdr" (dict "key" "" "value" .metaSpec.annotations "hdrOffset" (add $specOffset 4 | int)) -}}
{{- end -}}
{{- if .metaSpec -}}
{{- include "cHdr" (dict "key" "" "value" .metaSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}
{{- end -}}

{{- define "containerSpec" -}}
{{- $specOffset := .specOffset -}}
{{- printf "%s" "-" | nindent (add $specOffset 0 | int) -}}
{{- include "cHdr" (dict "key" "args" "value" .containerSpec.args "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "command" "value" .containerSpec.command "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "env" "value" .containerSpec.env "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "envFrom" "value" .containerSpec.envFrom "hdrOffset" (add $specOffset 2 | int)) -}}
{{- if (.nfVariables.root.Values.global) and (.nfVariables.root.Values.global.hub) -}}
{{- include "eHdr" (dict "key" "image" "value" (printf "%s/%s:%s" .nfVariables.root.Values.global.hub .containerSpec.image .containerSpec.tag) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- else if .containerSpec.hub -}}
{{- include "eHdr" (dict "key" "image" "value" (printf "%s/%s:%s" .containerSpec.hub .containerSpec.image .containerSpec.tag) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- else -}}
{{- include "eHdr" (dict "key" "image" "value" (printf "%s:%s" .containerSpec.image .containerSpec.tag) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}
{{- include "eHdr" (dict "key" "imagePullPolicy" "value" .containerSpec.imagePullPolicy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "lifecycle" "value" .containerSpec.lifecycle "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "livenessProbe" "value" .containerSpec.livenessProbe "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "name" "value" .containerSpec.name "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "ports" "value" .containerSpec.ports "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "readinessProbe" "value" .containerSpec.readinessProbe "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "resources" "value" .containerSpec.resources "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "securityContext" "value" .containerSpec.securityContext "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "stdin" "value" .containerSpec.stdin "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "stdinOnce" "value" .containerSpec.stdinOnce "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "terminationMessagePath" "value" .containerSpec.terminationMessagePath "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "terminationMessagePolicy" "value" .containerSpec.terminationMessagePolicy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "tty" "value" .containerSpec.tty "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "volumeDevices" "value" .containerSpec.volumeDevices "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "volumeMounts" "value" .containerSpec.volumeMounts "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "workingDir" "value" .containerSpec.workingDir "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "" "value" .containerSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "podSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "activeDeadlineSeconds" "value" .podSpec.activeDeadlineSeconds "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "affinity" "value" .podSpec.affinity "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "automountServiceAccountToken" "value" .podSpec.automountServiceAccountToken "hdrOffset" (add $specOffset 2 | int)) -}}
{{- $nfVariables := $.nfVariables -}}
{{- if .podSpec.containers -}}
{{- include "eHdr" (dict "key" "containers" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- range $index, $valueset := .podSpec.containers -}}
{{- include "containerSpec" (dict "specOffset" (add $specOffset 4 | int) "containerSpec" $valueset "nfVariables" $nfVariables) -}}
{{- end -}}
{{- end -}}
{{- include "cHdr" (dict "key" "dnsConfig" "value" .podSpec.dnsConfig "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "dnsPolicy" "value" .podSpec.dnsPolicy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "enableServiceLinks" "value" .podSpec.enableServiceLinks "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "hostAliases" "value" .podSpec.hostAliases "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "hostIPC" "value" .podSpec.hostIPC "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "hostNetwork" "value" .podSpec.hostNetwork "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "hostPID" "value" .podSpec.hostPID "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "hostname" "value" .podSpec.hostname "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "imagePullSecrets" "value" .podSpec.imagePullSecrets "hdrOffset" (add $specOffset 2 | int)) -}}
{{- if .podSpec.initContainers -}}
{{- include "eHdr" (dict "key" "initContainers" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- range $index, $valueset := .podSpec.initContainers -}}
{{- include "containerSpec" (dict "specOffset" (add $specOffset 4 | int) "containerSpec" $valueset "nfVariables" $nfVariables) -}}
{{- end -}}
{{- end -}}
{{- include "eHdr" (dict "key" "nodeName" "value" .podSpec.nodeName "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "nodeSelector" "value" .podSpec.nodeSelector "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "priority" "value" .podSpec.priority "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "priorityClassName" "value" .podSpec.priorityClassName "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "readinessGates" "value" .podSpec.readinessGates "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "restartPolicy" "value" .podSpec.restartPolicy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "runtimeClassName" "value" .podSpec.runtimeClassName "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "schedulerName" "value" .podSpec.schedulerName "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "securityContext" "value" .podSpec.securityContext "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "serviceAccount" "value" .podSpec.serviceAccount "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "serviceAccountName" "value" .podSpec.serviceAccountName "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "shareProcessNamespace" "value" .podSpec.shareProcessNamespace "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "subdomain" "value" .podSpec.subdomain "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "terminationGracePeriodSecond" "value" .podSpec.terminationGracePeriodSecond "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "tolerations" "value" .podSpec.tolerations "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "volumes" "value" .podSpec.volumes "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "" "value" .podSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "podTemplateSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "template" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "metaSpec" (dict "specOffset" (add $specOffset 2 | int) "metaSpec" .podTemplateSpec.metaSpec "nfVariables" .nfVariables) -}}
{{- include "podSpec" (dict "specOffset" (add $specOffset 2 | int) "podSpec" .podTemplateSpec.podSpec "nfVariables" .nfVariables) -}}
{{- include "cHdr" (dict "key" "" "value" .podTemplateSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "deploymentSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "minReadySeconds" "value" .deploymentSpec.minReadySeconds "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "paused" "value" .deploymentSpec.paused "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "progressDeadlineSeconds" "value" .deploymentSpec.progressDeadlineSeconds "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "replicas" "value" .deploymentSpec.replicas "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "revisionHistoryLimit" "value" .deploymentSpec.revisionHistoryLimit "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "selector" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "matchLabels" "value" .deploymentSpec.matchLabels "hdrOffset" (add $specOffset 4 | int)) -}}
{{- include "cHdr" (dict "key" "strategy" "value" .deploymentSpec.strategy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "podTemplateSpec" (dict "specOffset" (add $specOffset 2 | int) "podTemplateSpec" .deploymentSpec.podTemplateSpec "nfVariables" .nfVariables) -}}
{{- include "cHdr" (dict "key" "" "value" .deploymentSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "daemonsetSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "minReadySeconds" "value" .daemonsetSpec.minReadySeconds "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "revisionHistoryLimit" "value" .daemonsetSpec.revisionHistoryLimit "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "selector" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "matchLabels" "value" .daemonsetSpec.matchLabels "hdrOffset" (add $specOffset 4 | int)) -}}
{{- include "cHdr" (dict "key" "updatestrategy" "value" .daemonsetSpec.updatestrategy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "podTemplateSpec" (dict "specOffset" (add $specOffset 2 | int) "podTemplateSpec" .daemonsetSpec.podTemplateSpec "nfVariables" .nfVariables) -}}
{{- include "cHdr" (dict "key" "" "value" .daemonsetSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "pvcSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "accessModes" "value" (.pvcSpec.accessModes) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "dataSource" "value" (.pvcSpec.dataSource) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "resources" "value" (.pvcSpec.resources) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "eHdr" (dict "key" "selector" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "matchLabels" "value" (.pvcSpec.matchLabels) "hdrOffset" (add $specOffset 4 | int)) -}}
{{- include "eHdr" (dict "key" "storageClassName" "value" (.pvcSpec.storageClassName) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "volumeMode" "value" (.pvcSpec.volumeMode) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "volumeName" "value" (.pvcSpec.volumeName) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "" "value" .pvcSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "statefulsetSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "podManagementPolicy" "value" .statefulsetSpec.podManagementPolicy "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "replicas" "value" .statefulsetSpec.replicas "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "revisionHistoryLimit" "value" .statefulsetSpec.revisionHistoryLimit "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "selector" "value" " " "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "matchLabels" "value" .statefulsetSpec.matchLabels "hdrOffset" (add $specOffset 4 | int)) -}}
{{- include "eHdr" (dict "key" "serviceName" "value" .statefulsetSpec.serviceName "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "updatestrategy" "value" .statefulsetSpec.updatestrategy "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "podTemplateSpec" (dict "specOffset" (add $specOffset 2 | int) "podTemplateSpec" .statefulsetSpec.podTemplateSpec "nfVariables" .nfVariables) -}}
{{- include "cHdr" (dict "key" "volumeClaimTemplates" "value" .statefulsetSpec.volumeClaimTemplates "hdrOffset" (add $specOffset 2 | int)) -}}
{{- include "cHdr" (dict "key" "" "value" .statefulsetSpec.addHdrParams "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "clusterroleSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "cHdr" (dict "key" "aggregationRule" "value" (.clusterroleSpec.aggregationRule) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "rules" "value" (.clusterroleSpec.rules) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "" "value" (.clusterroleSpec.addHdrParams) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- end -}}

{{- define "clusterroleBindingSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "cHdr" (dict "key" "roleRef" "value" (.clusterroleBindingSpec.roleRef) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "subjects" "value" (.clusterroleBindingSpec.subjects) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- include "cHdr" (dict "key" "" "value" (.clusterroleBindingSpec.addHdrParams) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- end -}}

{{- define "serviceAccountSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "automountServiceAccountToken" "value" (.serviceAccountSpec.automountServiceAccountToken) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "imagePullSecrets" "value" (.serviceAccountSpec.imagePullSecrets) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- include "cHdr" (dict "key" "secrets" "value" (.serviceAccountSpec.secrets) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- include "cHdr" (dict "key" "" "value" (.serviceAccountSpec.addHdrParams) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- end -}}

{{- define "secretDataSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "data" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- (.nfVariables.root.Files.Glob "secret/*").AsSecrets | trimSuffix "\n" | nindent (add $specOffset 2 | int) }}
{{- end -}}

{{- define "secretSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "secretDataSpec" (dict "specOffset" (add $specOffset 0 | int) "secretDataSpec" ("") "nfVariables" .nfVariables) -}}
{{- include "cHdr" (dict "key" "stringData" "value" (.secretSpec.stringData) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "type" "value" (.secretSpec.type) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "" "value" (.secretSpec.addHdrParams) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- end -}}

{{- define "configMapDataSpec_2_9" -}}
{{- $specOffset := .specOffset -}}
{{- $nfVariables := $.nfVariables -}}
{{- include "eHdr" (dict "key" "data" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "__CFG_TYPE" "value" ($nfVariables.configtype|toString|quote) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- if eq ($.nfVariables.configtype|toString) "static-cfg" -}}
{{- $format := printf "%s" "config/static/*" -}}
{{- (.nfVariables.root.Files.Glob ($format|toString)).AsConfig | trimSuffix "\n" | nindent (add $specOffset 2 | int) }}
{{- end -}}
{{- if eq ($.nfVariables.configtype|toString) "env-cfg" -}}
{{- range $path, $_ :=  $nfVariables.root.Files.Glob  "config/env/*" -}}
{{- ($nfVariables.root.Files.Get $path) | trimSuffix "\n" | nindent (add $specOffset 2 | int) -}}
{{- end -}}
{{- end -}}
{{- if eq ($.nfVariables.configtype|toString) "mgmt-cfg" -}}
{{- include "eHdr" (dict "key" "revision" "value" "\"0\"" "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- $mgmtContents := printf "%s" "config/mgmt/*.*" -}}
{{- range $path, $_ := ($.nfVariables.root.Files.Glob ($mgmtContents|toString)) -}}
{{- $path | base | replace "@" "_rev_" | trimSuffix "\n" | nindent (add $specOffset 2 | int) }}: |-
{{- ($.nfVariables.root.Files.Get ( $path |toString)) | trimSuffix "\n" | nindent (add $specOffset 4 | int) }}
{{- end -}}
{{- end -}}
{{- if eq ($.nfVariables.configtype|toString) "dashboard-cfg" -}}
{{- $dataformat := printf "%s%s" "dashboard/*." ("json"|toString) -}}
{{- ($.nfVariables.root.Files.Glob ($dataformat|toString)).AsConfig | nindent (add $specOffset 2 | int) }}
{{- end -}}
{{- if eq ($.nfVariables.configtype|toString) "eventdef-cfg" -}}
{{- $dataformat := printf "%s%s" "eventdef/*." ("json"|toString) -}}
{{- ($.nfVariables.root.Files.Glob ($dataformat|toString)).AsConfig | nindent (add $specOffset 2 | int) }}
{{- end -}}
{{- if eq ($.nfVariables.configtype|toString) "alerts-cfg" -}}
{{- $dataformat := printf "%s%s" "alerts/*." ("yml"|toString) -}}
{{- ($.nfVariables.root.Files.Glob ($dataformat|toString)).AsConfig | nindent (add $specOffset 2 | int) }}
{{- end -}}
{{- if eq ($.nfVariables.configtype|toString) "metrics-cfg" -}}
{{- $dataformat := printf "%s%s" "metrics/*." ("yml"|toString) -}}
{{- ($.nfVariables.root.Files.Glob ($dataformat|toString)).AsConfig | nindent (add $specOffset 2 | int) }}
{{- end -}}
{{- end -}}

{{- define "configMapSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "cHdr" (dict "key" "binaryData" "value" (.configMapSpec.binaryData) "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "configMapDataSpec_2_9" (dict "specOffset" (add $specOffset 0 | int) "configMapDataSpec_2_9" ("") "nfVariables" .nfVariables) -}}
{{- include "cHdr" (dict "key" "" "value" (.configMapSpec.addHdrParams) "hdrOffset" (add $specOffset 0 | int)) -}}
{{- end -}}

{{- define "networkPolicySpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "egress" "value" (.networkPolicySpec.egress) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "ingress" "value" (.networkPolicySpec.ingress) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "podSelector" "value" (.networkPolicySpec.podSelector) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "policyTypes" "value" (.networkPolicySpec.policyTypes) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "" "value" (.networkPolicySpec.addHdrParams) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "ingressSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "cHdr" (dict "key" "backend" "value" (.ingressSpec.backend) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "rules" "value" (.ingressSpec.rules) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "tls" "value" (.ingressSpec.tls) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "" "value" (.ingressSpec.addHdrParams) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

{{- define "serviceSpec" -}}
{{- $specOffset := .specOffset -}}
{{- include "eHdr" (dict "key" "spec" "value" " " "hdrOffset" (add $specOffset 0 | int) ) -}}
{{- include "eHdr" (dict "key" "clusterIP" "value" (.serviceSpec.clusterIP) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "externalIPs" "value" (.serviceSpec.externalIPs) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "externalName" "value" (.serviceSpec.externalName) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "externalTrafficPolicy" "value" (.serviceSpec.externalTrafficPolicy) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "healthCheckNodePort" "value" (.serviceSpec.healthCheckNodePort) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "loadBalancerIP" "value" (.serviceSpec.loadBalancerIP) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "loadBalancerSourceRanges" "value" (.serviceSpec.loadBalancerSourceRanges) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "ports" "value" (.serviceSpec.ports) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "publishNotReadyAddresses" "value" (.serviceSpec.publishNotReadyAddresses) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "selector" "value" (.serviceSpec.selector) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "sessionAffinity" "value" (.serviceSpec.sessionAffinity) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "sessionAffinityConfig" "value" (.serviceSpec.sessionAffinityConfig) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "eHdr" (dict "key" "type" "value" (.serviceSpec.type) "hdrOffset" (add $specOffset 2 | int) ) -}}
{{- include "cHdr" (dict "key" "" "value" (.serviceSpec.addHdrParams) "hdrOffset" (add $specOffset 2 | int)) -}}
{{- end -}}

