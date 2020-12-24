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

package govel

import (
	//"client-go-master/rest"

	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var NetConfPort string
var RestConfPort string
var FmaasPort string

type AdditionalFields struct {
	ME_ID              string
	MGMT_IP            string
	CMAAS_NETCONF_PORT string
	APIGW_PORT         string
	FMAAS_HTTP_PORT    string
	APIGW_USERNAME     string
	APIGW_PASSWORD     string
	APIGW_AUTH_TYPE    string
	COLLECTOR_ID       string
}

type heartbeatFields struct {
	AdditionalFields       AdditionalFields `json:"additionalFields,omitempty"`
	HeartbeatFieldsVersion string           `json:"heartbeatFieldsVersion"`
	HeartbeatInterval      int              `json:"heartbeatInterval"`
}

// HeartbeatEvent with optional field block for fields specific to heartbeat events
type HeartbeatEvent struct {
	EventHeader     `json:"commonEventHeader"`
	heartbeatFields `json:"heartbeatFields,omitempty"`
}
type EventHeartbeatSet HeartbeatEvent

// NewHeartbeat creates a new heartbeat event
func NewHeartbeat(id, name, eType, sourceName string, interval int, eventType string) *HeartbeatEvent {

	// var kubeconfig = ""

	// config, err := getConfig(kubeconfig)
	// if err != nil {
	// 	log.Fatal("Failed to load client config: %v", err)
	// 	// return nil, err
	// }

	// //config, _ := clientcmd.BuildConfigFromFlags("", "C:\\Users\\sharmav\\go\\bin\\admin.conf")

	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	panic(err.Error())
	// }

	// nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	// if err != nil {
	// 	panic(err)
	// }
	// nodeip := []corev1.NodeAddress{}
	// nodeip = nodes.Items[0].Status.Addresses

	hb := new(HeartbeatEvent)
	hb.Domain = DomainHeartbeat
	hb.Priority = PriorityNormal
	hb.Version = "4.1"
	hb.EventID = id
	hb.EventName = name
	//hb.SourceName = sourceName // Removed for PRD#R018
	//      hb.EventName = "Heartbeat_<namespace of VES or NF >" // Change PRD# MultiNF
	//hb.EventType = "<namespace of VES or NF>:<VES service name>" // Change PRD#R014
	hb.EventType = eType
	//hb.ReportingEntityID = "POD id" // Change PRD#R015
	//hb.ReportingEntityName = hb.EventType // Change PRD#R016
	//hb.SourceID = hb.ReportingEntityID // Change PRD#R017
	//hb.SourceName = hb.ReportingEntityName // Change PRD#R018

	hb.StartEpochMicrosec = time.Now().UnixNano() / 1000
	hb.LastEpochMicrosec = hb.StartEpochMicrosec

	//hb fields
	hb.HeartbeatFieldsVersion = "3.0"
	hb.VesEventListenerVersion = "7.1"
	hb.HeartbeatInterval = interval
	//	hb.AdditionalFields.MGMT_IP = nodeip[0].Address
	return hb
}

func NewHeartbeatCollector(id, name, eType, sourceName string, interval int) *HeartbeatEvent {

	hbc := new(HeartbeatEvent)
	hbc.Domain = DomainHeartbeat
	hbc.Priority = PriorityNormal
	hbc.Version = "4.1"
	hbc.EventID = id
	hbc.EventName = name

	hbc.EventType = eType

	hbc.StartEpochMicrosec = time.Now().UnixNano() / 1000
	hbc.LastEpochMicrosec = hbc.StartEpochMicrosec

	//hb fields
	hbc.HeartbeatFieldsVersion = "3.0"
	hbc.VesEventListenerVersion = "7.1"
	hbc.HeartbeatInterval = interval
	//	hb.AdditionalFields.MGMT_IP = nodeip[0].Address
	return hbc
}

func getConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
