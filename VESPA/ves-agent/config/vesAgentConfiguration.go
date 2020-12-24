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

package config

import (
	"VESPA/govel"
	// "github.com/nokia/onap-vespa/govel"
)

var DefaultConf VESAgentConfiguration

type CollectorDetails struct {
	Id               int64                        `json:"id"`
	PrimaryCollector govel.CollectorConfiguration `json:"primaryCollector"`
	BackupCollector  govel.CollectorConfiguration `json:"backupCollector,omitempty"`
}

var (
	VesCollectorObjectslist = make(map[KeyStruct]int64)
)

type CollectorList struct {
	CollectorId []CollectorId `json:"CollectorId"`
}

type CollectorId struct {
	Id int64 `json:"id"`
}

type DomainCollectors struct {
	Fault        CollectorList `json:"fault"`
	Tca          CollectorList `json:"tca"`
	Measurment   CollectorList `json:"measurment"`
	Notificaiton CollectorList `json:"notificaiton"`
}

type KeyStruct struct {
	PrimaryFQDN string
	PrimaryPort int
	BackupFQDN  string
	BackupPort  int
}

// VESAgentConfiguration parameters
type VESAgentConfiguration struct {
	CollectorDetails []*CollectorDetails        `json:"collectorDetails"`
	Domains          *DomainCollectors          `json:"domains"`
	Heartbeat        *HeartbeatConfiguration    `json:"heartbeat,omitempty"`
	Measurement      *MeasurementConfiguration  `json:"measurement,omitempty"`
	Event            *govel.EventConfiguration  `json:"event,omitempty"`
	AlertManager     *AlertManagerConfiguration `json:"alertManager,omitempty"`
	Cluster          *ClusterConfiguration      `json:"cluster"` // Optional cluster config. If absent, fallbacks to single node mode
	Debug            bool                       `mapstructure:"debug,omitempty"`
	LogLevel         string                     `mapstructure:"loglevel,omitempty"`
	CaCert           string                     `mapstructure:"caCert,omitempty"` // Root certificate content
	DataDir          string                     `mapsctructure:"datadir"`         // Path to directory containing data
	XGVelaInfo       XGVelaInfo                 `mapsctructure:"XGVelaInfo"`
	KafkaAddress     []string                   `mapsctructure:"kafkaAddress"`
	// NbiFormat        string                    `mapsctructure:"nbiFormat"`
	Topic string `mapsctructure:"topic"`
	//NameSpace string `mapsctructure:"nameSpace"`
}

type XGVelaInfo struct {
	DnPrefix         string `mapstructure:"dnPrefix,omitempty"`
	XGVelaID         string `mapstructure:"xgvelaId,omitempty"`
	CmaasNetconfPort string `mapstructure:"CMAAS_NETCONF_PORT,omitempty"`
	FmaasHttpPort    string `mapstructure:"FMAAS_HTTP_PORT,omitempty"`
	MgmtIp           string `mapstructure:"MGMT_IP,omitempty"`
}
