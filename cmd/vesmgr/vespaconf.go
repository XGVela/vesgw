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
	"time"
)

// Structs are copied from https://github.com/nokia/ONAP-VESPA/tree/master/ves-agent/config
// and from https://github.com/nokia/ONAP-VESPA/blob/master/govel/config.go
// Using tag v0.3.0

// HeartbeatConfiguration parameters
type HeartbeatConfiguration struct {
	DefaultInterval time.Duration `yaml:"defaultInterval"`
}

// Label represents a VES field by it's name, with an expression
// for getting its value
type Label struct {
	Name string `yaml:"name"`
	Expr string `yaml:"expr"`
}

// MetricRule defines how to retrieve metrics and map them
// into a list of evel.EventMeasurement struct
type MetricRule struct {
	Target         string  `yaml:"target"`          // Target VES event field
	Expr           string  `yaml:"expr"`            // Prometheus query expression
	VMIDLabel      string  `yaml:"vmId"`            // Metric label holding the VNF ID
	Labels         []Label `yaml:"labels"`          // Set of VES fields to map to values of given label
	ObjectName     string  `yaml:"object_name"`     // JSON Object Name
	ObjectInstance string  `yaml:"object_instance"` // JSON Object instance
	ObjectKeys     []Label `yaml:"object_keys"`     // JSON Object keys
}

// MetricRules defines a list of rules, and defaults values for them
type MetricRules struct {
	DefaultValues *MetricRule  `yaml:"defaults"` // Default rules to apply (except for expr), labels are merged
	Metrics       []MetricRule `yaml:"metrics"`  // List of query and mapping of rules
}

// PrometheusConfig parameters
type PrometheusConfig struct {
	Address   string        `yaml:"address"`   // Base URL to prometheus API
	Timeout   time.Duration `yaml:"timeout"`   // API request timeout
	KeepAlive time.Duration `yaml:"keepalive"` // HTTP Keep-Alive
	Rules     MetricRules   `yaml:"rules"`     // Querying rules
}

// MeasurementConfiguration parameters
type MeasurementConfiguration struct {
	DomainAbbreviation   string           `yaml:"domainAbbreviation"`   // "Measurement" or "Mfvs"
	DefaultInterval      time.Duration    `yaml:"defaultInterval"`      // Default measurement interval
	MaxBufferingDuration time.Duration    `yaml:"maxBufferingDuration"` // Maximum timeframe size of buffering
	Prometheus           PrometheusConfig `yaml:"prometheus"`           // Prometheus configuration
}

// CollectorConfiguration parameters
type CollectorConfiguration struct {
	ServerRoot string `yaml:"serverRoot"`
	FQDN       string `yaml:"fqdn"`
	Port       int    `yaml:"port"`
	Secure     bool   `yaml:"secure"`
	Topic      string `yaml:"topic"`
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	PassPhrase string `yaml:"passphrase,omitempty"` // passPhrase used to encrypt collector password in file
}

//NfcNamingCode mapping bettween NfcNamingCode (oam or etl) and Vnfcs
type NfcNamingCode struct {
	Type  string   `yaml:"type"`
	Vnfcs []string `yaml:"vnfcs"`
}

// EventConfiguration parameters
type EventConfiguration struct {
	VNFName             string          `yaml:"vnfName"`             // Name of this VNF, eg: dpa2bhsxp5001v
	ReportingEntityName string          `yaml:"reportingEntityName"` // Value of reporting entity field. Usually local VM (VNFC) name
	ReportingEntityID   string          `yaml:"reportingEntityID"`   // Value of reporting entity UUID. Usually local VM (VNFC) UUID
	MaxSize             int             `yaml:"maxSize"`
	NfNamingCode        string          `yaml:"nfNamingCode,omitempty"` // "hspx"
	NfcNamingCodes      []NfcNamingCode `yaml:"nfcNamingCodes,omitempty"`
	RetryInterval       time.Duration   `yaml:"retryInterval,omitempty"`
	MaxMissed           int             `yaml:"maxMissed,omitempty"`
}

// VESAgentConfiguration parameters
type VESAgentConfiguration struct {
	PrimaryCollector CollectorConfiguration   `yaml:"primaryCollector"`
	Heartbeat        HeartbeatConfiguration   `yaml:"heartbeat,omitempty"`
	Measurement      MeasurementConfiguration `yaml:"measurement,omitempty"`
	Event            EventConfiguration       `yaml:"event,omitempty"`
	Debug            bool                     `yaml:"debug,omitempty"`
	CaCert           string                   `yaml:"caCert,omitempty"` // Root certificate content
	DataDir          string                   `yaml:"datadir"`          // Path to directory containing data
}
