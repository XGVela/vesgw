/*
	Copyright 2019 Nokia
	Copyright (c) 2020 Mavenir

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/
package configmapreader

import (
	logging "VESPA/cimUtils"
	"VESPA/govel"
	"VESPA/ves-agent/agent"
	"VESPA/ves-agent/config"

	"fmt"
	// "log"
	"os"
	"strings"
	"time"

	"encoding/json"
	"syscall"

	uuid "VESPA/ves-agent/go.uuid"

	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watch "k8s.io/apimachinery/pkg/watch"
)

const (
	metricsSuffix = "-metrics-cfg"
	msvcLabel     = "microSvcName"
	eventSuffix   = "-eventdef-cfg"
	mgmtSuffix    = "-mgmt-cfg"
	//NfsFolder     = "./nfs/"
)

var (
	cmapData        v1.ConfigMap
	msConfigmaps    map[string]map[string]string
	msAnnotations   map[string]map[string]*govel.TmaaSAnnotations
	tmaasAnnotation *govel.TmaaSAnnotations
	err             error
	metricNew       config.MetricRules
)

// Metrics struct definition
type Metrics struct {
	Metrics ConfigMapDetails `yaml:"metrics,omitempty"`
}

// ConfigMapDetails struct definition
type ConfigMapDetails struct {
	AdditionalObjects      []config.MetricRule `yaml:"AdditionalObjects,omitempty"`
	AdditionalMeasurements []config.MetricRule `yaml:"AdditionalMeasurements,omitempty"`
}

func GetServicePortCMaas() {

	client, _ := NewKubeConfig()
	/*options := metaV1.ListOptions{
		LabelSelector: labels.Set{"microSvcName": "config-service"}.AsSelector().String(),
	}*/

	//labelSelector := metaV1.LabelSelector{MatchLabels: map[string]string{"microSvcName": "config-service"}}
	listOptions := metaV1.ListOptions{
		//LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		LabelSelector: "microSvcName=config-service",
		Limit:         100,
	}
	services, _ := client.Client.CoreV1().Services(os.Getenv("K8S_NAMESPACE")).List(listOptions)
	fmt.Println("Services Length=====>>>>>>>>>>", len(services.Items))
	for _, svc := range services.Items {
		fmt.Println("Services Cmaas=====>>>>>>>>>>", svc)
		for i, _ := range svc.Spec.Ports {
			fmt.Println("Service Port Name=====>>>>>>>>>>", svc.Spec.Ports[i].Name)
			if svc.Spec.Ports[i].Name == "netconf" {
				str := fmt.Sprint(svc.Spec.Ports[i].NodePort)
				fmt.Println("NetConfPort===========>>>>>>>>>>>str", str)
				govel.NetConfPort = str
			}
			if svc.Spec.Ports[i].Name == "restconf" {
				str := fmt.Sprint(svc.Spec.Ports[i].NodePort)
				fmt.Println("RestConfPort===========>>>>>>>>>>>str", str)
				govel.RestConfPort = str
				fmt.Println("RestConfPort=====>>>>>>>>>>", govel.RestConfPort)
			}
		}
	}

}
func GetServicePortFMaas() {

	client, _ := NewKubeConfig()
	/*options := metaV1.ListOptions{
		LabelSelector: labels.Set{"microSvcName": "fault-service"}.AsSelector().String(),
	}*/
	//labelSelector := metaV1.LabelSelector{MatchLabels: map[string]string{"microSvcName": "fault-service"}}
	listOptions := metaV1.ListOptions{
		//LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		LabelSelector: "microSvcName=fault-service",
		Limit:         100,
	}
	services, _ := client.Client.CoreV1().Services(os.Getenv("K8S_NAMESPACE")).List(listOptions)
	fmt.Println("Services Fmaas=====>>>>>>>>>>", services)
	for _, svc := range services.Items {
		fmt.Println("Service FMAAS=====>>>>>>>>>>", svc)
		for i, _ := range svc.Spec.Ports {
			fmt.Println("Service Port Name=====>>>>>>>>>>", svc.Spec.Ports[i].Name)
			if svc.Spec.Ports[i].Name == "rest" {
				str := fmt.Sprint(svc.Spec.Ports[i].NodePort)
				fmt.Println("FmaasPort===========>>>>>>>>>>>str", str)
				govel.FmaasPort = str
				fmt.Println("FmaasPort=====>>>>>>>>>>", govel.FmaasPort)

			}
		}
	}

}

//ConfigMapWatcher watch all the metric configmaps
func ConfigMapWatcher() {

	msConfigmaps = make(map[string]map[string]string)
	msAnnotations = make(map[string]map[string]*govel.TmaaSAnnotations)
	go RunUpdateJob()
	client, err := NewKubeConfig()
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "error getting K8s client:"+err.Error())
		return
	}

	for {
		watcher, err := client.Client.CoreV1().ConfigMaps(metaV1.NamespaceAll).Watch(metaV1.ListOptions{})
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "error in getting configmap watcher:"+err.Error())

			return
		}
		configMapChanel := watcher.ResultChan()
		logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "connected to configmap watch channel")

		for event := range configMapChanel {
			configMap, ok := event.Object.(*v1.ConfigMap)

			if !ok {
				logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "unexpected type")

			} else {
				labels := configMap.GetLabels()

				microsvc := labels[msvcLabel]

				if strings.HasSuffix(configMap.GetName(), metricsSuffix) || strings.HasSuffix(configMap.GetName(), eventSuffix) || strings.HasSuffix(configMap.GetName(), mgmtSuffix) {

					annotations := configMap.GetAnnotations()

					if annotations["xgvela.com/tmaas"] == "" {
						logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "annotation not found in configmap")

						continue
					} else {
						tmaasAnnotation, err = getAnnotations(annotations["xgvela.com/tmaas"])
						if err != nil {
							logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "error while reading annotation of "+fmt.Sprintf("%s", microsvc))

							continue
						}
					}
					logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", " Configmap "+fmt.Sprintf("%v", event.Type)+" with namespace: "+fmt.Sprintf("%v", configMap.GetNamespace())+" name:"+fmt.Sprintf("%s", configMap.GetName())+" microSvcName:"+fmt.Sprintf("%s", microsvc))

					switch event.Type {
					case watch.Added, watch.Modified:
						fmt.Println("XGVela ID from tmaas annotation for NF", tmaasAnnotation.XGVelaID)
						fmt.Println("XGVela ID for VESGW", govel.VesgwXGVelaId)
						if tmaasAnnotation.XGVelaID == govel.VesgwXGVelaId {
							fmt.Println("Creating Map for XGVela ID ", tmaasAnnotation.XGVelaID)
							go UpdateMetricRuleMap(configMap, microsvc, tmaasAnnotation)
						}
					case watch.Deleted:
						go deleteNfFile(configMap)
					}
				}
			}
		}
		logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "ConfigMapWatcher", "configmap watch connection dropped, reconnecting...")
	}

}

func deleteNfFile(cmapData *v1.ConfigMap) {
	namespace := cmapData.GetNamespace()
	path := "./nfs/" + namespace + ".yaml"
	err := os.Remove(path)
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "deleteNfFile", "error while deleting the file:"+fmt.Sprintf("%v", path)+err.Error())
		return
	}
	// stop the current running schedulers
	agent.StopScheduler(path)
}

// UpdateMetricRuleMap updates internal map when called by watcher
func UpdateMetricRuleMap(cmapData *v1.ConfigMap, microsvc string, tmaasAnns *govel.TmaaSAnnotations) {

	namespace := cmapData.GetNamespace()
	logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "UpdateMetricRuleMap", "Updating configmap for nf :"+fmt.Sprintf("%s", namespace))

	_, ok := msConfigmaps[namespace]
	if !ok {
		msConfigmaps[namespace] = make(map[string]string)
	}

	_, ok = msAnnotations[namespace]
	if !ok {
		msAnnotations[namespace] = make(map[string]*govel.TmaaSAnnotations)
	}

	for key, value := range cmapData.Data {
		if key == "metrics.yml" || key == "metricss.yml" {
			msConfigmaps[namespace][microsvc] = value
			msAnnotations[namespace][microsvc] = tmaasAnns
			logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "UpdateMetricRuleMap", "Updated NF with value: "+fmt.Sprintf("%s", key)+fmt.Sprintf("%v", tmaasAnns)+fmt.Sprintf("%v", microsvc)+fmt.Sprintf("%v", namespace)+fmt.Sprintf("%v", value))
		} else {
			_, ok := msConfigmaps[namespace][microsvc]
			if !ok {
				msConfigmaps[namespace][microsvc] = ""
			}
			_, ok = msAnnotations[namespace][microsvc]
			if !ok {
				msAnnotations[namespace][microsvc] = tmaasAnns
			}
		}
	}

}

// RunUpdateJob checks for the recent updates
func RunUpdateJob() {
	interval := 10
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	for range ticker.C {
		UpdateRules()
	}
}

//UpdateRules update rules in the nf level metric rule file
func UpdateRules() {
	defer func() {
		// clear map
		for key := range msConfigmaps {
			delete(msConfigmaps, key)
		}
	}()

	if len(msConfigmaps) != 0 {
		for namespace, metricDetails := range msConfigmaps {
			var ruleConfig config.NFMetricConfigRulesWithoutNfc
			err := config.ReadNFMetricFile(namespace, &ruleConfig)
			//nf file not exist create and write metric rules
			if err != nil && err.Error() == "fileNotFound" {
				err1 := nfFileFirstWrite(namespace, metricDetails, &ruleConfig)
				if err1 != nil {
					logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "UpdateMetricRuleMap", "error while writing into a file "+err1.Error())

					continue
				}
				continue

			}

			for msName, updatedDetails := range metricDetails {
				if updatedDetails == "" {
					continue
				}
				for k := range ruleConfig.Measurement.Prometheus.Rules {
					if msName == ruleConfig.Measurement.Prometheus.Rules[k].MicroserviceName {

						var updatedData *Metrics

						err := yaml.Unmarshal([]byte(updatedDetails), &updatedData)
						if err != nil {
							logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "UpdateMetricRuleMap", "error while unmarshalling the updated details"+err.Error())

							continue
						}

						if updatedData.Metrics.AdditionalObjects != nil {

							ruleConfig.Measurement.Prometheus.Rules[k].MicroserviceName = msName
							ruleConfig.Measurement.Prometheus.Rules[k].Defaults = &config.MetricRule{
								VMIDLabel: namespace + "_" + msName,
							}

							for i := range updatedData.Metrics.AdditionalObjects {
								updatedData.Metrics.AdditionalObjects[i].Target = "AdditionalObjects"

							}

							//fmt.Printf("updated additionalObject details: %+v", updatedData.Metrics.AdditionalObjects)
							ruleConfig.Measurement.Prometheus.Rules[k].Metrics = updatedData.Metrics.AdditionalObjects

						}
						logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "UpdateMetricRuleMap", "updated values are: "+fmt.Sprintf("%v", ruleConfig))
					}

				}
			}

			fileName := "./nfs/" + namespace + ".yaml"
			// //update nf file
			ApplyPatch(fileName, &ruleConfig)

			//start processing collection of metrics
			go agent.StartNfAgentProcessing("MODIFIED", fileName)

		}

	}
}

//ApplyPatch update the config file
func ApplyPatch(fileName string, ruleConfig *config.NFMetricConfigRulesWithoutNfc) {

	data, err := yaml.Marshal(&ruleConfig)
	if err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "configmapreader", "ApplyPatch", "error: %v"+err.Error())

	}

	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "ApplyPatch", "error while checking if config file exists: %v"+err.Error())
	} else {
		err := os.Remove(fileName)
		if err != nil {
			logging.LogForwarder("EXCEPTION", syscall.Gettid(), "configmapreader", "ApplyPatch", "error while deleting existing config file: %v"+err.Error())
		}
	}

	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "configmapreader", "ApplyPatch", err.Error())
	}

	_, err = f.Write(data)
	if err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "configmapreader", "ApplyPatch", err.Error())
	}

	f.Close()
	logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "ApplyPatch", "file "+fmt.Sprintf("%s", fileName)+" is updated successfully")

}

func nfFileFirstWrite(namespace string, metricDetails map[string]string, ruleConfig *config.NFMetricConfigRulesWithoutNfc) error {

	createDirIfNotExist("./nfs/")

	//update the default config for prometheus
	//hardcoded values as of now.
	ruleConfig.Measurement.DomainAbbreviation = "Mvfs"
	ruleConfig.Measurement.DefaultInterval = config.DefaultConf.Measurement.DefaultInterval
	ruleConfig.Measurement.MaxBufferingDuration = config.DefaultConf.Measurement.MaxBufferingDuration
	ruleConfig.Measurement.Prometheus.Address = config.DefaultConf.Measurement.Prometheus.Address
	ruleConfig.Measurement.Prometheus.Timeout = config.DefaultConf.Measurement.Prometheus.Timeout
	ruleConfig.Measurement.Prometheus.KeepAlive = config.DefaultConf.Measurement.Prometheus.KeepAlive

	ruleConfig.Event.VNFName = namespace
	ruleConfig.Event.DnPrefix = config.DefaultConf.XGVelaInfo.DnPrefix

	ruleConfig.Event.MaxSize = config.DefaultConf.Event.MaxSize

	ruleConfig.Event.RetryInterval = config.DefaultConf.Event.RetryInterval
	var k = 0
	for msName, updatedDetails := range metricDetails {
		// logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "nfFileFirstWrite", "INSIDE FOR LOOP "+fmt.Sprintf("%s", msName))

		k++
		// logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "nfFileFirstWrite", "recieved details are:"+fmt.Sprintf("%v", updatedDetails))

		var updatedData *Metrics
		err := yaml.Unmarshal([]byte(updatedDetails), &updatedData)
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "nfFileFirstWrite", "error while unmarshalling the updated details", err.Error())
			return err
		}
		logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "nfFileFirstWrite", "updatedDetails: "+fmt.Sprintf("%v", updatedData))

		if updatedData == nil {
			ruleConfig.Event.Tmaas = msAnnotations[namespace][msName]
			logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "nfFileFirstWrite", fmt.Sprintf("ruleConfig.Event.Tmaas : %v", ruleConfig.Event.Tmaas))

			ruleConfig.Event.SourceName = ruleConfig.Event.DnPrefix + ",ManagedElement=me-" + ruleConfig.Event.Tmaas.XGVelaID + ",NetworkFunction=" + ruleConfig.Event.Tmaas.NfID

			reportingEntityName := ruleConfig.Event.DnPrefix + ",ManagedElement=me-" + ruleConfig.Event.Tmaas.XGVelaID + ",NetworkFunction=" + ruleConfig.Event.Tmaas.XGVelaID

			ruleConfig.Event.ReportingEntityName = reportingEntityName
			ruleConfig.Event.NfNamingCode = ruleConfig.Event.Tmaas.NfType
			// ruleConfig.Event.NfcNamingCodes = ruleConfig.Event.Tmaas.NfServiceType
			// ruleConfig.Measurement.Prometheus
			nfcUuid := generateUUID(reportingEntityName)
			ruleConfig.Event.ReportingEntityID = nfcUuid
			ruleConfig.Event.MaxMissed = config.DefaultConf.Event.MaxMissed

		} else if updatedData.Metrics.AdditionalObjects != nil {

			// var metric config.MetricRules

			metricNew.MicroserviceName = msName
			metricNew.Defaults = &config.MetricRule{
				VMIDLabel: namespace + "_" + msName,
			}

			for i := range updatedData.Metrics.AdditionalObjects {
				updatedData.Metrics.AdditionalObjects[i].Target = "AdditionalObjects"

			}

			//fmt.Printf("updated additionalObject details: %+v", updatedData.Metrics.AdditionalObjects)
			metricNew.Metrics = updatedData.Metrics.AdditionalObjects
			// ruleConfig.Measurement.Prometheus.Rules = append(ruleConfig.Measurement.Prometheus.Rules, metricNew)
			ruleConfig.Event.Tmaas = msAnnotations[namespace][msName]
			metricNew.NfcNamingCodes = ruleConfig.Event.Tmaas.NfServiceType
			ruleConfig.Measurement.Prometheus.Rules = append(ruleConfig.Measurement.Prometheus.Rules, metricNew)

			fmt.Println("ruleConfig.Event.Tmaas: ", ruleConfig.Event.Tmaas)
			ruleConfig.Event.SourceName = ruleConfig.Event.DnPrefix + ",ManagedElement=me-" + ruleConfig.Event.Tmaas.XGVelaID + ",NetworkFunction=" + ruleConfig.Event.Tmaas.NfID

			reportingEntityName := ruleConfig.Event.DnPrefix + ",ManagedElement=me-" + ruleConfig.Event.Tmaas.XGVelaID + ",NetworkFunction=" + ruleConfig.Event.Tmaas.XGVelaID

			ruleConfig.Event.ReportingEntityName = reportingEntityName
			ruleConfig.Event.NfNamingCode = ruleConfig.Event.Tmaas.NfType
			//metric.NfcNamingCodes = ruleConfig.Event.Tmaas.NfServiceType
			//  ruleConfig.Event.NfcNamingCodes = ruleConfig.Event.Tmaas.NfServiceType
			nfcUuid := generateUUID(reportingEntityName)
			ruleConfig.Event.ReportingEntityID = nfcUuid
			ruleConfig.Event.MaxMissed = config.DefaultConf.Event.MaxMissed

		}

	}

	logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "nfFileFirstWrite", "updated values of ruleconfig: "+fmt.Sprintf("%v", ruleConfig))

	fileName := "./nfs/" + namespace + ".yaml"
	// //update nf file
	ApplyPatch(fileName, ruleConfig)

	//start processing collection of metrics
	go agent.StartNfAgentProcessing("ADDED", fileName)
	return nil
}

func createDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func getAnnotations(tmaasVals string) (*govel.TmaaSAnnotations, error) {

	var tmaasAnns *govel.TmaaSAnnotations

	logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "getAnnotations", "tmaas Annotations", tmaasVals)
	err = json.Unmarshal([]byte(tmaasVals), &tmaasAnns)
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "configmapreader", "getAnnotations", "Annotation spec unmarshall fail")

		return nil, err
	}

	// logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "getAnnotations", fmt.Sprintf("Json Data after unmarshal %v", tmaasAnns))

	return tmaasAnns, nil
}

func generateUUID(reportingEntityName string) string {
	newId := uuid.NewV3(reportingEntityName).String()
	logging.LogForwarder("INFO", syscall.Gettid(), "configmapreader", "generateUUID", "uuid generated is: "+fmt.Sprintf("%s", newId))

	return newId
}
