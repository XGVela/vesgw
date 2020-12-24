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

import "VESPA/govel"

var (
	VesConfigOb *VesConfigObj
)

type VesConfigObj struct {
	Config *Config `json:"config"`
}

type Config struct {
	CollectorDetail []*CollectorDetail         `json:"collectorDetails"`
	Domains         *Vesdomains                `json:"vesdomains"`
	Heartbeat       *Heartbeat                 `json:"heartbeat"`
	Measurement     *Measurement               `json:"measurement"`
	Event           *Event                     `json:"event"`
	AlertManager    *AlertManagerConfiguration `json:"alertManager"`
	Cluster         *ClusterConfiguration      `json:"cluster"` // Optional cluster config. If absent, fallbacks to single node mode
}

type CollectorDetail struct {
	Id               int64      `json:"id"`
	PrimaryCollector *Collector `json:"primaryCollector"`
	BackupCollector  *Collector `json:"backupCollector"`
}

type Collector struct {
	//ServerRoot string   `json:"serverRoot,omitempty"`
	Fqdn       string   `json:"fqdn"`
	Port       int      `json:"port"`
	Secure     bool     `json:"secure"`
	User       string   `json:"user"`
	Password   string   `json:"password"`
	Passphrase string   `json:"passphrase"`
	NbiType    string   `json:"nbiType"`
	Brokers    []string `json:"kafkaBrokers"`
	Topic      string   `json:"topic"`
	KafkaTopic string   `json:"kafkaTopic"`
	NbiFormat  string   `json:"nbiFormat"`
	HeartBeat  bool     `json:"heartbeat"`
}

type Vesdomains struct {
	Fault        VESGWDomains `json:"fault"`
	Measurement  VESGWDomains `json:"measurement"`
	Notification VESGWDomains `json:"notification"`
	Tca          VESGWDomains `json:"tca"`
}

type VESGWDomains struct {
	CollectorLists []*CollectorLists `json:"collectorList"`
}
type CollectorLists struct {
	Id int64 `json:"id"`
}

type Heartbeat struct {
	DefaultInterval  string           `json:"defaultInterval"`
	AdditionalFields AdditionalFields `json:"additionalFields"`
}

type Measurement struct {
	DomainAbbreviation   string      `json:"domainAbbreviation"`
	DefaultInterval      string      `json:"defaultInterval"`
	MaxBufferingDuration string      `json:"maxBufferingDuration"`
	Prometheus           *Prometheus `json:"prometheus"`
}

type Prometheus struct {
	Address   string        `json:"address"`
	Timeout   string        `json:"timeout"`
	Keepalive string        `json:"keepalive"`
	Rules     []MetricRules `json:"rules"`
}

type Event struct {
	VNFName             string                  `json:"vnfName"`             // Name of this VNF, eg: dpa2bhsxp5001v
	ReportingEntityName string                  `json:"reportingEntityName"` // Value of reporting entity field. Usually local VM (VNFC) name
	ReportingEntityID   string                  `json:"reportingEntityID"`   // Value of reporting entity UUID. Usually local VM (VNFC) UUID
	MaxSize             int                     `json:"maxSize"`
	NfNamingCode        string                  `json:"nfNamingCode"` // "hspx"
	NfcNamingCodes      string                  `json:"nfcNamingCodes"`
	RetryInterval       string                  `json:"retryInterval"`
	MaxMissed           int                     `json:"maxMissed"`
	Tmaas               *govel.TmaaSAnnotations `json:"tmaas"`
	DnPrefix            string                  `json:"dnPrefix"`
	SourceName          string                  `json:"sourceName"`
}

type AlertManager struct {
	Bind string `json:"bind"`
}

type Cluster struct {
	Debug       bool `json:"debug"`
	DisplayLogs bool `json:"displayLogs"`
}
