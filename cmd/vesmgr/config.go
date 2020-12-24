/*
 *  Copyright (c) 2019 AT&T Intellectual Property.
 *  Copyright (c) 2018-2019 Nokia.
 *  Copyright (c) 2020 Mavenir.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

const defaultReportingEntityID = "00000000-0000-0000-0000-000000000000"
const defaultVNFName = "Vespa"
const defaultNFNamingCode = "ricp"

func readSystemUUID() string {
	data, err := ioutil.ReadFile("/sys/class/dmi/id/product_uuid")
	if err != nil {
		return defaultReportingEntityID
	}
	return strings.TrimSpace(string(data))
}

func getVNFName() string {
	VNFName := os.Getenv("VESMGR_VNFNAME")
	if VNFName == "" {
		return defaultVNFName
	}
	return VNFName
}

func getNFNamingCode() string {
	NFNamingCode := os.Getenv("VESMGR_NFNAMINGCODE")
	if NFNamingCode == "" {
		return defaultNFNamingCode
	}
	return NFNamingCode
}

func basicVespaConf() VESAgentConfiguration {
	var vespaconf = VESAgentConfiguration{
		DataDir: "/tmp/data",
		Debug:   false,
		Event: EventConfiguration{
			VNFName:             getVNFName(),
			ReportingEntityName: "Vespa",
			ReportingEntityID:   readSystemUUID(),
			MaxSize:             2000000,
			NfNamingCode:        getNFNamingCode(),
			NfcNamingCodes:      []NfcNamingCode{},
			RetryInterval:       time.Second * 5,
			MaxMissed:           2,
		},
		Measurement: MeasurementConfiguration{
			// Domain abbreviation has to be set to “Mvfs” for VES 5.3,
			// and to “Measurement” for later VES interface versions.
			DomainAbbreviation:   "Mvfs",
			MaxBufferingDuration: time.Hour,
			Prometheus: PrometheusConfig{
				Timeout:   time.Second * 30,
				KeepAlive: time.Second * 30,
				Rules: MetricRules{
					DefaultValues: &MetricRule{
						VMIDLabel: "'{{.labels.instance}}'",
					},
				},
			},
		},
	}
	return vespaconf
}

// AppMetricsStruct contains xapplication metrics definition
type AppMetricsStruct struct {
	ObjectName     string
	ObjectInstance string
}

// AppMetrics contains metrics definitions for all Xapps
type AppMetrics map[string]AppMetricsStruct

// Parses the metrics data from an array of bytes, which is expected to contain a JSON
// array with structs of the following format:
//
// { ...
//   "config" : {
//     "metrics": [
//       { "name": "...", "objectName": "...", "objectInstamce": "..." },
//       ...
//     ]
//   }
// }
func parseMetricsFromXAppDescriptor(descriptor []byte, appMetrics AppMetrics) AppMetrics {
	var desc []map[string]interface{}
	json.Unmarshal(descriptor, &desc)

	for _, app := range desc {
		config, configOk := app["config"]
		if configOk {
			metrics, metricsOk := config.(map[string]interface{})["metrics"]
			if metricsOk {
				parseMetricsRules(metrics.([]interface{}), appMetrics)
			}
		}
	}
	return appMetrics
}

// Parses the metrics data from an array of interfaces, which are expected to be maps
// of the following format:
//    { "name": xxx, "objectName": yyy, "objectInstance": zzz }
// Entries, which do not have all the necessary fields, are ignored.
func parseMetricsRules(metricsMap []interface{}, appMetrics AppMetrics) AppMetrics {
	for _, element := range metricsMap {
		name, nameOk := element.(map[string]interface{})["name"].(string)
		if nameOk {
			_, alreadyFound := appMetrics[name]
			objectName, objectNameOk := element.(map[string]interface{})["objectName"].(string)
			objectInstance, objectInstanceOk := element.(map[string]interface{})["objectInstance"].(string)
			if !alreadyFound && objectNameOk && objectInstanceOk {
				appMetrics[name] = AppMetricsStruct{objectName, objectInstance}
				logger.Info("parsed counter %s %s %s", name, objectName, objectInstance)
			}
			if alreadyFound {
				logger.Info("skipped duplicate counter %s", name)
			}
		}
	}
	return appMetrics
}

func getRules(vespaconf *VESAgentConfiguration, xAppConfig []byte) {
	appMetrics := make(AppMetrics)
	parseMetricsFromXAppDescriptor(xAppConfig, appMetrics)

	makeRule := func(expr string, objName string, objInstance string) MetricRule {
		return MetricRule{
			Target:         "AdditionalObjects",
			Expr:           expr,
			ObjectInstance: objInstance,
			ObjectName:     objName,
			ObjectKeys: []Label{
				Label{
					Name: "ricComponentName",
					Expr: "'{{.labels.kubernetes_name}}'",
				},
			},
		}
	}
	var metricsMap map[string][]interface{}
	json.Unmarshal(xAppConfig, &metricsMap)
	metrics := parseMetricsRules(metricsMap["metrics"], appMetrics)

	vespaconf.Measurement.Prometheus.Rules.Metrics = make([]MetricRule, 0, len(metrics))
	for key, value := range metrics {
		vespaconf.Measurement.Prometheus.Rules.Metrics = append(vespaconf.Measurement.Prometheus.Rules.Metrics, makeRule(key, value.ObjectName, value.ObjectInstance))
	}
	if len(vespaconf.Measurement.Prometheus.Rules.Metrics) == 0 {
		logger.Info("vespa config with empty metrics")
	}
}

func getCollectorConfiguration(vespaconf *VESAgentConfiguration) {
	vespaconf.PrimaryCollector.User = os.Getenv("VESMGR_PRICOLLECTOR_USER")
	vespaconf.PrimaryCollector.Password = os.Getenv("VESMGR_PRICOLLECTOR_PASSWORD")
	vespaconf.PrimaryCollector.PassPhrase = os.Getenv("VESMGR_PRICOLLECTOR_PASSPHRASE")
	vespaconf.PrimaryCollector.FQDN = os.Getenv("VESMGR_PRICOLLECTOR_ADDR")
	vespaconf.PrimaryCollector.ServerRoot = os.Getenv("VESMGR_PRICOLLECTOR_SERVERROOT")
	vespaconf.PrimaryCollector.Topic = os.Getenv("VESMGR_PRICOLLECTOR_TOPIC")
	portStr := os.Getenv("VESMGR_PRICOLLECTOR_PORT")
	if portStr == "" {
		vespaconf.PrimaryCollector.Port = 8443
	} else {
		port, _ := strconv.Atoi(portStr)
		vespaconf.PrimaryCollector.Port = port
	}
	secureStr := os.Getenv("VESMGR_PRICOLLECTOR_SECURE")
	if secureStr == "true" {
		vespaconf.PrimaryCollector.Secure = true
	} else {
		vespaconf.PrimaryCollector.Secure = false
	}
}

func createVespaConfig(writer io.Writer, xAppStatus []byte) {
	vespaconf := basicVespaConf()
	getRules(&vespaconf, xAppStatus)
	getCollectorConfiguration(&vespaconf)
	err := yaml.NewEncoder(writer).Encode(vespaconf)
	if err != nil {
		logger.Error("Cannot write vespa conf file: %s", err.Error())
		return
	}
}
