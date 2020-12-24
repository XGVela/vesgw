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

// EventDomain is the kind of event
type EventDomain string

// List of possible values for EventDomain
const (
	DomainFault                  EventDomain = "fault"
	DomainHeartbeat              EventDomain = "heartbeat"
	DomainMeasurements           EventDomain = "measurement"
	DomainMobileFlow             EventDomain = "mobileFlow"
	DomainNotification           EventDomain = "notification"
	DomainOther                  EventDomain = "other"
	DomainPerf3gpp               EventDomain = "perf3gpp"
	DomainPnfRegistration        EventDomain = "pnfRegistration"
	DomainSipSignaling           EventDomain = "sipSignaling"
	DomainStateChange            EventDomain = "stateChange"
	DomainSyslog                 EventDomain = "syslog"
	DomainThresholdCrossingAlert EventDomain = "thresholdCrossingAlert"
	DomainVoiceQuality           EventDomain = "voiceQuality"
)

// EventPriority is the event's level of priority
type EventPriority string

// Possible values for EventPriority
const (
	PriorityHigh   EventPriority = "High"
	PriorityMedium EventPriority = "Medium"
	PriorityNormal EventPriority = "Normal"
	PriorityLow    EventPriority = "Low"
)

type ReplayCollectorInfo struct {
	InitReplay        bool
	ReplayCollectorId int64
	LastEventId       int64
	ReplayOnGoing     bool // response to FMAAS
	StartIndex        int64
}

// Event has commont methods for VES Events  structures
type Event interface {
	// Header returns a reference to the event's commonEventHeader
	Header() *EventHeader
}

// Batch is a list of events
type Batch []Event

// Split extracts cut the batch into 2 batches of equal length ( +/- 1)
func (batch Batch) Split() (Batch, Batch) {
	i := len(batch) / 2
	return batch[:i], batch[i:]
}

// Len returns the number of events in the batch
func (batch Batch) Len() int {
	return len(batch)
}

// UpdateReportingEntityName will update `reportingEntityName` field
// on events of batch for which this field has no value. Other events
// which already have the field set will be left untouched
func (batch Batch) UpdateReportingEntityName(name string) {
	for _, evt := range batch {
		if evt.Header().ReportingEntityName == "" {
			evt.Header().ReportingEntityName = name
		}
	}
}

// UpdateReportingEntityID will update `reportingEntityID` field
// on events of batch for which this field has no value. Other events
// which already have the field set will be left untouched
func (batch Batch) UpdateReportingEntityID(id string) {
	for _, evt := range batch {
		if evt.Header().ReportingEntityID == "" {
			evt.Header().ReportingEntityID = id
		}
	}
}

// EventHeader is the common part of all kind of events
type EventHeader struct {
	Domain                  EventDomain   `json:"domain"`
	EventID                 string        `json:"eventId"`
	EventName               string        `json:"eventName"`
	EventType               string        `json:"eventType,omitempty"`
	InternalHeaderFields    interface{}   `json:"internalHeaderFields,omitempty"`
	LastEpochMicrosec       int64         `json:"lastEpochMicrosec, omitempty"`
	NfNamingCode            string        `json:"nfNamingCode,omitempty"`
	NfcNamingCode           string        `json:"nfcNamingCode,omitempty"`
	NfVendorName            string        `json:"nfVendorName,omitempty"`
	Priority                EventPriority `json:"priority"`
	ReportingEntityID       string        `json:"reportingEntityId,omitempty"`
	ReportingEntityName     string        `json:"reportingEntityName"`
	Sequence                int64         `json:"sequence"`
	SourceID                string        `json:"sourceId,omitempty"`
	SourceName              string        `json:"sourceName"`
	StartEpochMicrosec      int64         `json:"startEpochMicrosec"`
	TimeZoneOffset          string        `json:"timeZoneOffset,omitempty"`
	Version                 string        `json:"version"`
	VesEventListenerVersion string        `json:"vesEventListenerVersion"`
}

// EventHeader is the common part of all kind of events
type RCPEventHeader struct {
	Sequence  int64 `json:"sequenceId"`
	EventTime int64 `json:"eventTime,omitempty"`
	//SysUpTime      time.Time `json:"sysUpTime"`
	AlarmId       string `json:"alarmId"`
	ManagedObject string `json:"managedObject"`
	AlarmName     string `json:"alarmName"`
	// EquipmentSubId string    `json:"equipmentSubId"`
	// ProbableCause  string    `json:"probableCause"`
	Severity  string `json:"severity"`
	AlarmCode string `json:"alarmCode"`
	Uhn       string `json:"uhn"`
	Cnfc_uui  string `json:"cnfc_uui"`
	Pod_uuid  string `json:"pod_uuid"`
}

type Header struct {
	Sequence  int64 `json:"sequence"`
	EventTime int64 `json:"eventTime,omitempty"`
	// SysUpTime      int64  `json:"sysUpTime"`
	AlarmId       string `json:"alarmId"`
	ManagedObject string `json:"managedObject"`
	AlarmName     string `json:"alarmName"`
	// EquipmentSubId string `json:"equipmentSubId"`
	// ProbableCause  string `json:"probableCause"`
	Severity string `json:"severity"`
}

// EventHeader is the common part of all kind of events
type NewEventHeader struct {
	Domain                  EventDomain   `json:"domain"`
	EventID                 string        `json:"eventId"`
	EventName               string        `json:"eventName"`
	EventType               string        `json:"eventType,omitempty"`
	InternalHeaderFields    interface{}   `json:"internalHeaderFields,omitempty"`
	LastEpochMillis         int64         `json:"lastEpochMillis, omitempty "`
	NfNamingCode            string        `json:"nfNamingCode,omitempty"`
	NfcNamingCode           string        `json:"nfcNamingCode,omitempty"`
	NfVendorName            string        `json:"nfVendorName,omitempty"`
	Priority                EventPriority `json:"priority"`
	ReportingEntityID       string        `json:"reportingEntityId,omitempty"`
	ReportingEntityName     string        `json:"reportingEntityName"`
	Sequence                int64         `json:"sequence"`
	SourceID                string        `json:"sourceId,omitempty"`
	SourceName              string        `json:"sourceName"`
	StartEpochMillis        int64         `json:"startEpochMillis"`
	TimeZoneOffset          string        `json:"timeZoneOffset,omitempty"`
	Version                 string        `json:"version"`
	VesEventListenerVersion string        `json:"vesEventListenerVersion"`
	LocalEventName          string        `json:"localEventName,omitempty"`
}

// Header returns a reference self
func (hdr *EventHeader) Header() *EventHeader {
	return hdr
}

// EventField is used for additional events fields
type EventField struct {
	// Name of the field
	Name string `json:"key"`
	// Value of the field
	Value string `json:"value"`
}

type Counter struct {
	Criticality      Criticality `json:"criticality"`
	HashMap          interface{} `json:"hashMap"`
	ThresholdCrossed string      `json:"thresholdCrossed"`
}

type ReplayFields struct {
	CollectorID int64 `json:"collectorId"`
	StartIndex  int64 `json:"start"`
	EndIndex    int64 `json:"end,omitempty"`
}

type ReplayResponse struct {
	CollectorStatus string `json:"collectorStatus"`
	ReplayOnGoing   bool   `json:"replayOngoing"`
	LastSequenceID  int64  `json:"internalLastSequence"`
}
