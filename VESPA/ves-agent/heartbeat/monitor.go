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

package heartbeat

import (
	logging "VESPA/cimUtils"
	"VESPA/govel"
	"VESPA/ves-agent/config"

	"fmt"
	// "log"
	"syscall"
	"time"

	uuid "VESPA/ves-agent/go.uuid"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type annotationStruct struct {
	VendorID      string `yaml:"vendorId,omitempty"`
	XGVelaID      string `yaml:"xgvelaId,omitempty"`
	NfClass       string `yaml:"nfClass,omitempty"`
	NfType        string `yaml:"nfType,omitempty"`
	NfID          string `yaml:"nfId,omitempty"`
	NfServiceID   string `yaml:"nfServiceId,omitempty"`
	nfServiceType string `yaml:"nfServiceId,omitempty"`
}

// MonitorState handles the monitor internal state
type MonitorState interface {
	// NextHeartbeatIndex return the next event index and increments it
	NextHeartbeatIndex() (int64, error)
}

type inMemState struct {
	index int64
}

func (mem *inMemState) NextHeartbeatIndex() (int64, error) {
	i := mem.index
	mem.index++
	return i, nil
}

// Monitor is an utility to create heartbeats
type Monitor struct {
	state    MonitorState // Monitor internal state
	eventCfg *govel.EventConfigurationWithoutNfc
}

// NewMonitorWithState creates a new Heartbeat Monitor from provided configuration
// and provided state handler
func NewMonitorWithState(conf *govel.EventConfigurationWithoutNfc, state MonitorState) (*Monitor, error) {
	return &Monitor{state: state, eventCfg: conf}, nil
}

// NewMonitor creates a new Heartbeat Monitor from provided configuration
// that use an in memory state
func NewMonitor(conf *govel.EventConfigurationWithoutNfc) (*Monitor, error) {
	return NewMonitorWithState(conf, &inMemState{index: 0})
}

// Run creates a new Heartbeat
func (mon *Monitor) Run(from, to time.Time, interval time.Duration) (interface{}, error) {
	idx, err := mon.state.NextHeartbeatIndex()
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("heartbeat%.10d", idx)
	eventName := "heartbeat_" + mon.eventCfg.NfNamingCode + "-" + mon.eventCfg.Tmaas.VendorID
	// nfNamingCode := mon.eventCfg.NfNamingCode
	// eventName := "heartbeat_" + mon.eventCfg.NfNamingCode
	//eventType := mon.nfNamingCode + mon.sourceName
	//eventType := mon.eventCfg.VNFName
	eventType := "HEARTBEAT"
	hb := govel.NewHeartbeat(id, eventName, eventType, mon.eventCfg.VNFName, int(interval.Seconds()), eventType)
	hb.NfNamingCode = mon.eventCfg.NfNamingCode
	hb.NfcNamingCode = mon.eventCfg.NfNamingCode
	hb.ReportingEntityName = config.DefaultConf.Heartbeat.AdditionalFields.DnPrefix + mon.eventCfg.ReportingEntityName
	hb.ReportingEntityID = mon.eventCfg.ReportingEntityID
	hb.SourceName = config.DefaultConf.Heartbeat.AdditionalFields.DnPrefix + mon.eventCfg.SourceName
	//hb.SourceID = mon.eventCfg.ReportingEntityID
	hb.SourceID = generateUUID(hb.SourceName)
	hb.NfVendorName = mon.eventCfg.Tmaas.VendorID
	//hb.TimeZoneOffset = in UTC

	// hb.AdditionalFields.CMAAS_NETCONF_PORT =
	// hb.AdditionalFields.FMAAS_HTTP_PORT = config.DefaultConf.Heartbeat.AdditionalFields.FmaasHttpPort
	hb.AdditionalFields.ME_ID = generateUUID(config.DefaultConf.Heartbeat.AdditionalFields.DnPrefix + ",ManagedElement=me-" + config.DefaultConf.Heartbeat.AdditionalFields.XGVelaID)

	if config.DefaultConf.Heartbeat.AdditionalFields.MgmtIp != "0.0.0.0" {
		hb.AdditionalFields.MGMT_IP = config.DefaultConf.Heartbeat.AdditionalFields.MgmtIp
	} else {
		var kubeconfig = ""

		config, err := getConfig(kubeconfig)
		if err != nil {
			logging.LogForwarder("EXCEPTION", syscall.Gettid(), "heartbeat", "Run", "Failed to load client config: %v"+err.Error())

			// return nil, err
		}

		//config, _ := clientcmd.BuildConfigFromFlags("", "C:\\Users\\sharmav\\go\\bin\\admin.conf")

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			logging.LogForwarder("EXCEPTION", syscall.Gettid(), "heartbeat", "Run", err.Error())

		}

		nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		nodeip := []corev1.NodeAddress{}
		nodeip = nodes.Items[0].Status.Addresses
		hb.AdditionalFields.MGMT_IP = nodeip[0].Address
	}

	return hb, nil
}

func generateUUID(reportingEntityName string) string {
	newId := uuid.NewV3(reportingEntityName).String()
	logging.LogForwarder("INFO", syscall.Gettid(), "heartbeat", "generateUUID", "uuid generated is:"+fmt.Sprintf("%s", newId))

	return newId
}

func getConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
