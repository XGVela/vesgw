/*
	Copyright 2019 Nokia
	Copyright (c) 2020 Mavenir.

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
	"strings"
	"time"
)

// Severity for faults
type Severity string

// Possible values for Severity
const (
	SeverityCritical Severity = "CRITICAL"
	SeverityMajor    Severity = "MAJOR"
	SeverityMinor    Severity = "MINOR"
	SeverityWarning  Severity = "WARNING"
	SeverityNormal   Severity = "NORMAL"
)

type Criticality string

const (
	CriticalityCrit Criticality = "CRIT"
	CriticalityMaj  Criticality = "MAJ"
)

// VfStatus is the virtual function status
type VfStatus string

// Possible values for VfStatus
const (
	StatusActive           VfStatus = "Active"
	StatusIdle             VfStatus = "Idle"
	StatusPrepTerminate    VfStatus = "Preparing to terminate"
	StatusReadyTerminate   VfStatus = "Ready to terminate"
	StatusRequestTerminate VfStatus = "Requesting termination"
)

// SourceType of the fault originator
type SourceType string

// Possible values for SourceType
const (
	SourceOther                  SourceType = "other"
	SourceRouter                 SourceType = "router"
	SourceSwitch                 SourceType = "switch"
	SourceHost                   SourceType = "host"
	SourceCard                   SourceType = "card"
	SourcePort                   SourceType = "port"
	SourceSlotThreshold          SourceType = "slotThreshold"
	SourcePortThreshold          SourceType = "portThreshold"
	SourceVirtualMachine         SourceType = "virtualMachine"
	SourceVirtualNetworkFunction SourceType = "virtualNetworkFunction"
)

// Data is the data passed to notification templates and webhook pushes.
type Data struct {
	Receiver          string    `json:"receiver"`
	Status            string    `json:"status"`
	Alerts            Alerts    `json:"alerts"`
	MavData           EventType `json:"event"`
	GroupLabels       KV        `json:"groupLabels"`
	CommonLabels      KV        `json:"commonLabels"`
	CommonAnnotations KV        `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

type RCPData struct {
	Receiver          string       `json:"receiver"`
	Status            string       `json:"status"`
	Alerts            Alerts       `json:"alerts"`
	MavData           RCPEventType `json:"event"`
	GroupLabels       KV           `json:"groupLabels"`
	CommonLabels      KV           `json:"commonLabels"`
	CommonAnnotations KV           `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

// Alert holds one alert for notification templates.
type Alert struct {
	Status       string    `json:"status"`
	Labels       KV        `json:"labels"`
	Annotations  KV        `json:"annotations"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	GeneratorURL string    `json:"generatorURL"`
	Fingerprint  string    `json:"fingerprint"`
}

// Alerts is a list of Alert objects.
type Alerts []Alert

// DataTca is a list of TCA objects.
type DataTca []Data

// KV is a set of key/value string pairs.
type KV map[string]string

type faultFields struct {
	AlarmAdditionalInformation interface{} `json:"alarmAdditionalInformation"`
	AlarmCondition             string      `json:"alarmCondition"`
	AlarmInterfaceA            string      `json:"alarmInterfaceA,omitempty"`
	EventCategory              string      `json:"eventCategory,omitempty"`
	EventFaultSeverity         Severity    `json:"eventSeverity"`
	EventSourceType            string      `json:"eventSourceType"`
	FaultFieldsVersion         string      `json:"faultFieldsVersion"`
	SpecificProblem            string      `json:"specificProblem"`
	VfStatus                   VfStatus    `json:"vfStatus"`
}

type RcpFaultFields struct {
	AlarmAdditionalInformation interface{} `json:"alarmAdditionalInformation"`
	AlarmCondition             string      `json:"alarmCondition"`
	AlarmInterfaceA            string      `json:"alarmInterfaceA,omitempty"`
	EventCategory              string      `json:"eventCategory,omitempty"`
	EventFaultSeverity         Severity    `json:"eventSeverity"`
	EventSourceType            string      `json:"eventSourceType"`
	FaultFieldsVersion         string      `json:"faultFieldsVersion"`
	SpecificProblem            string      `json:"specificProblem"`
	VfStatus                   VfStatus    `json:"vfStatus"`
}

//ThresholdCrossingAlertFields is a fault event
type thresholdCrossingAlertFields struct {
	AdditionalFields               interface{} `json:"additionalFields,omitempty"`
	AdditionalParameters           []Counter   `json:"additionalParameters"`
	AlertAction                    string      `json:"alertAction"`
	AlertDescription               string      `json:"alertDescription"`
	AlertType                      string      `json:"alertType"`
	CollectionTimestamp            string      `json:"collectionTimestamp"`
	EventSeverity                  string      `json:"eventSeverity"`
	EventStartTimestamp            string      `json:"eventStartTimestamp"`
	ThresholdCrossingFieldsVersion string      `json:"thresholdCrossingFieldsVersion"`
}

type RcpThresholdCrossingAlertFields struct {
	AdditionalFields interface{} `json:"additionalFields,omitempty"`
	// AdditionalParameters           []Counter   `json:"additionalParameters"`
	AlertAction                    string `json:"alertAction"`
	AlertDescription               string `json:"alertDescription"`
	AlertType                      string `json:"alertType"`
	CollectionTimestamp            string `json:"collectionTimestamp"`
	EventSeverity                  string `json:"eventSeverity"`
	EventStartTimestamp            string `json:"eventStartTimestamp"`
	ThresholdCrossingFieldsVersion string `json:"thresholdCrossingFieldsVersion"`
}

type notificationFields struct {
	AdditionalFields          interface{} `json:"additionalFields,omitempty"`
	ChangeContact             string      `json:"changeContact, omitempty"`
	ChangeIdentifier          string      `json:"changeIdentifier"`
	ChangeType                string      `json:"changeType"`
	NotificationFieldsVersion string      `json:"notificationFieldsVersion"`
	NewState                  string      `json:"newState, omitempty"`
	OldState                  string      `json:"oldState, omitempty"`
	StateInterface            string      `json:"stateInterface, omitempty"`
}

type RcpnotificationFields struct {
	AdditionalFields          interface{} `json:"additionalFields,omitempty"`
	ChangeContact             string      `json:"changeContact, omitempty"`
	ChangeIdentifier          string      `json:"changeIdentifier"`
	ChangeType                string      `json:"changeType"`
	NotificationFieldsVersion string      `json:"notificationFieldsVersion"`
	NewState                  string      `json:"newState, omitempty"`
	OldState                  string      `json:"oldState, omitempty"`
	StateInterface            string      `json:"stateInterface, omitempty"`
	ProductType               string      `json:"productType"`
}

//type RCPnotificationFields struct {//
//Body interface{} `json:"body, omitempty"`
//}
type RCPFaultFields struct {
	Body interface{} `json:"body, omitempty"`
}

type RCPnotificationFields struct {
	Body interface{} `json:"body, omitempty"`
}

type RCPTCAFields struct {
	Body interface{} `json:"body, omitempty"`
}

type RCPAdditionalFields struct {
	MeID                string `json:"meId, omitempty"`
	MeLabel             string `json:"meLabel, omitempty"`
	MsUID               string `json:"msuid, omitempty"`
	NFId                string `json:"nfId, omitempty"`
	NFLabel             string `json:"nfLabel, omitempty"`
	NFName              string `json:"nfName, omitempty"`
	NFServiceID         string `json:"nfServiceId, omitempty, omitempty"`
	NfServiceInstanceID string `json:"nfServiceInstanceId, omitempty"`
	NfServiceType       string `json:"nfServiceType, omitempty"`
	NfSwVersion         string `json:"nfSwVersion, omitempty"`
	NfType              string `json:"nfType, omitempty"`
	ProbableCause       string `json:"probableCause, omitempty"`
}

type RCPEventBody struct {
	ChangeContact             string `json:"changeContact, omitempty"`
	ChangeIdentifier          string `json:"changeIdentifier"`
	ChangeType                string `json:"changeType"`
	NotificationFieldsVersion string `json:"notificationFieldsVersion"`
	NewState                  string `json:"newState, omitempty"`
	OldState                  string `json:"oldState, omitempty"`
	StateInterface            string `json:"stateInterface, omitempty"`
	ProductType               string `json:"productType"`
}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type RCPFaultBody struct {
	AlarmCondition     string   `json:"alarmCondition"`
	AlarmInterfaceA    string   `json:"alarmInterfaceA,omitempty"`
	EventCategory      string   `json:"eventCategory,omitempty"`
	EventFaultSeverity Severity `json:"eventSeverity"`
	EventSourceType    string   `json:"eventSourceType"`
	FaultFieldsVersion string   `json:"faultFieldsVersion"`
	SpecificProblem    string   `json:"specificProblem"`
	VfStatus           VfStatus `json:"vfStatus"`
	ProductType        string   `json:"productType"`
}

type RCPTcaBody struct {
	//AdditionalParameters           []Counter `json:"additionalParameters"`
	AlertAction                    string `json:"alertAction"`
	AlertDescription               string `json:"alertDescription"`
	AlertType                      string `json:"alertType"`
	CollectionTimestamp            string `json:"collectionTimestamp"`
	EventSeverity                  string `json:"eventSeverity"`
	EventStartTimestamp            string `json:"eventStartTimestamp"`
	ThresholdCrossingFieldsVersion string `json:"thresholdCrossingFieldsVersion"`
	ProductType                    string `json:"productType"`
}

//EventFault is a fault event
type EventFault struct {
	EventHeader        `json:"commonEventHeader"`
	faultFieldSeverity `json:"faultFields"`
}

type faultFieldSeverity struct {
	AlarmAdditionalInformation interface{} `json:"alarmAdditionalInformation,omitempty"`
	AlarmCondition             string      `json:"alarmCondition"`
	AlarmInterfaceA            string      `json:"alarmInterfaceA,omitempty"`
	EventCategory              string      `json:"eventCategory,omitempty"`
	EventSeverity              Severity    `json:"eventSeverity"`
	EventSourceType            string      `json:"eventSourceType"`
	FaultFieldsVersion         string      `json:"faultFieldsVersion"`
	SpecificProblem            string      `json:"specificProblem"`
	VfStatus                   VfStatus    `json:"vfStatus"`
}

//EventTCA is a fault event
type EventTCA struct {
	EventHeader                  `json:"commonEventHeader"`
	thresholdCrossingAlertFields `json:"thresholdCrossingAlertFields"`
}

//EventNotification is a fault event
type EventNotification struct {
	EventHeader        `json:"commonEventHeader"`
	notificationFields `json:"notificationFields"`
}

//EventNotification is a fault event
type RCPEventNotification struct {
	RCPEventHeader `json:"header"`
	RCPnotificationFields
}
type RCPEventTCA struct {
	RCPEventHeader `json:"header"`
	RCPTCAFields
}

//EventFault is a fault event
type RCPEventFault struct {
	RCPEventHeader `json:"header"`
	RCPFaultFields
}

//EventType is a fault event
type EventType struct {
	NewEventHeader               `json:"commonEventHeader"`
	faultFields                  `json:"faultFields"`
	thresholdCrossingAlertFields `json:"thresholdCrossingAlertFields"`
	NotificationFields           notificationFields `json:"notificationFields"`
}

type RCPEventType struct {
	NewEventHeader                  `json:"commonEventHeader"`
	RcpFaultFields                  `json:"faultFields"`
	RcpThresholdCrossingAlertFields `json:"thresholdCrossingAlertFields"`
	RcpNotificationFields           RcpnotificationFields `json:"notificationFields"`
}
type RCPEnhancedAdditionalFields struct {
	Uhn           string `json:"uhn"`
	Cnfc_uui      string `json:"cnfc_uuid"`
	Pod_uuid      string `json:"pod_uuid"`
	CorrelationId string `json:"correlationId"`
}

// NewFault creates a new fault event
func NewFault(name, publisherName, id, condition string, eventType string, reportingEnId string, reportingEnintyName string, sequence int64, sourceId string, specificProblem string, priority EventPriority, severity Severity, status VfStatus, sourceName string, lastEpochMicrosec int64, startEpochMicrosec int64, eventCategory string, eventSourceType string, nfNamingcode string, nfcNamingcode string) *EventFault {
	fault := new(EventFault)
	fault.AlarmCondition = condition
	fault.SpecificProblem = specificProblem
	if severity == "CLEAR" {
		fault.EventSeverity = "NORMAL"
	} else {
		fault.EventSeverity = severity
	}
	fault.VfStatus = status
	fault.FaultFieldsVersion = "4.0"

	fault.Domain = DomainFault
	fault.EventID = id
	// fault.EventName = name + "_" + publisherName + "_" + condition
	fault.EventName = name
	//fault.EventType = fault.EventSourceType 		// Change PRD#R006
	fault.EventType = eventType
	fault.ReportingEntityID = reportingEnId
	fault.Sequence = sequence
	//fault.ReportingEntityID = 						// Change PRD#R007
	//fault.ReportingEntityName = fault.EventType		// Change PRD#R008
	fault.ReportingEntityName = reportingEnintyName // Change PRD#R008
	fault.SourceID = sourceId                       // Change PRD#R009
	fault.SourceName = sourceName

	fault.Version = "4.1"
	fault.Priority = priority
	fault.VesEventListenerVersion = "7.1"

	fault.StartEpochMicrosec = startEpochMicrosec * 1000
	fault.LastEpochMicrosec = lastEpochMicrosec * 1000
	fault.EventCategory = eventCategory
	fault.EventSourceType = eventSourceType
	fault.NfNamingCode = nfNamingcode
	fault.NfcNamingCode = nfcNamingcode

	return fault
}

func NewFaultForRCP(sequence int64, startEpochMicrosec int64, lastEpochMicrosec int64, id, reportingEnintyName string, name string, severity Severity, localEventName string) *RCPEventFault {

	fault := new(RCPEventFault)
	fault.Sequence = sequence
	startep := startEpochMicrosec / 1000000
	fault.EventTime = startep
	// lastepoch := time.Unix(0, lastEpochMicrosec*int64(time.Millisecond))
	// fault.SysUpTime = lastepoch
	fault.AlarmId = id
	fault.ManagedObject = reportingEnintyName
	fault.AlarmName = name
	// fault.EquipmentSubId = "xyz"
	// fault.ProbableCause = "Disk Usage"
	if severity == "CLEAR" {
		fault.Severity = "Clear"
	} else {
		faultSeverity := string(severity)
		fault.Severity = strings.Title(strings.ToLower(faultSeverity))
	}
	fault.AlarmCode = localEventName

	return fault
}

func NewTCAForRCP(sequence int64, startEpochMicrosec int64, lastEpochMicrosec int64, id, reportingEnintyName string, name string, severity string, localEventName string) *RCPEventTCA {

	tca := new(RCPEventTCA)
	tca.Sequence = sequence
	startep := startEpochMicrosec / 1000000
	tca.EventTime = startep
	// lastepoch := time.Unix(0, lastEpochMicrosec*int64(time.Millisecond))
	// tca.SysUpTime = lastepoch
	tca.AlarmId = id
	tca.ManagedObject = reportingEnintyName
	tca.AlarmName = name
	// tca.EquipmentSubId = "xyz"
	// tca.ProbableCause = "Disk Usage"
	if severity == "CLEAR" {
		tca.Severity = "Clear"
	} else {
		tca.Severity = strings.Title(strings.ToLower(severity))
	}
	tca.AlarmCode = localEventName

	return tca
}

// NewTCA creates a new fault event
// func NewTCA(name, id, condition, specificProblem string, priority EventPriority, severity Severity, sourceType SourceType, status VfStatus, sourceName string) *EventFault {
func NewTCA(AdditionalP []Counter, alertA, alertD, alertT, collectionT, eventS, eventStartT, thresholdC, name string, publisherName string, id string, condition string, eventType string, reportingEnId string, reportingEnintyName string, sequence int64, sourceId string, priority EventPriority, sourceName string, status VfStatus, lastEpochMicrosec int64, startEpochMicrosec int64, nfNamingcode string, nfcNamingcode string) *EventTCA {
	tca := new(EventTCA)

	tca.AdditionalParameters = AdditionalP
	tca.AlertAction = alertA
	tca.AlertDescription = alertD
	tca.AlertType = alertT
	tca.CollectionTimestamp = collectionT
	if eventS == "CLEAR" {
		tca.EventSeverity = "NORMAL"
	} else {
		tca.EventSeverity = eventS
	}
	tca.EventStartTimestamp = eventStartT
	tca.ThresholdCrossingFieldsVersion = thresholdC

	tca.Domain = DomainThresholdCrossingAlert
	tca.EventID = id
	// tca.EventName = name + "_" + publisherName + "_" + condition
	tca.EventName = name

	tca.Sequence = sequence
	tca.EventType = eventType
	tca.ReportingEntityID = reportingEnId         // Change PRD#R040
	tca.ReportingEntityName = reportingEnintyName // Change PRD#R041
	tca.SourceID = sourceId                       // Change PRD#R042
	tca.SourceName = sourceName

	tca.Version = "4.1"
	tca.Priority = priority
	tca.VesEventListenerVersion = "7.1"

	tca.StartEpochMicrosec = startEpochMicrosec * 1000
	tca.LastEpochMicrosec = lastEpochMicrosec * 1000
	tca.NfNamingCode = nfNamingcode
	tca.NfcNamingCode = nfcNamingcode

	return tca
}

func NewNotificationForRCP(sequence int64, startEpochMicrosec int64, lastEpochMicrosec int64, id, reportingEnintyName string, name string, priority EventPriority, localEventName string) *RCPEventNotification {

	notification := new(RCPEventNotification)
	notification.Sequence = sequence
	//startepoch := time.Unix(0, startEpochMicrosec*int64(time.Millisecond))
	startep := startEpochMicrosec / 1000000
	notification.EventTime = startep
	//lastepoch := time.Unix(0, lastEpochMicrosec*int64(time.Millisecond))
	//notification.SysUpTime = lastepoch
	notification.AlarmId = id
	notification.ManagedObject = reportingEnintyName
	notification.AlarmName = name
	//	notification.EquipmentSubId = "xyz"
	//	notification.ProbableCause = "Disk Usage"
	notification.Severity = "Notification"
	notification.AlarmCode = localEventName
	return notification
}

// NewNotification creates a new fault event
// func NewNotification
func NewNotification(ChangeContact string, changeIdentifier string, changeType string, notificationFieldsVersion string, newState string, oldState string, stateInterface string, name string, id string, eventType string, reportingEnId string, reportingEnintyName string, sequence int64, sourceId string, priority EventPriority, sourceName string, status VfStatus, lastEpochMicrosec int64, startEpochMicrosec int64, nfNamingcode string, nfcNamingcode string) *EventNotification {
	notification := new(EventNotification)
	notification.ChangeContact = ChangeContact
	notification.ChangeIdentifier = changeIdentifier
	notification.ChangeType = changeType
	notification.NotificationFieldsVersion = notificationFieldsVersion
	notification.NewState = newState
	notification.OldState = oldState
	notification.StateInterface = stateInterface

	notification.Domain = DomainNotification
	notification.SourceName = sourceName
	notification.EventName = name
	notification.EventID = id
	notification.EventType = eventType
	notification.Sequence = sequence
	notification.ReportingEntityID = reportingEnId
	notification.ReportingEntityName = reportingEnintyName
	notification.SourceID = sourceId
	notification.Version = "4.1"
	notification.Priority = priority
	notification.VesEventListenerVersion = "7.1"
	notification.StartEpochMicrosec = startEpochMicrosec * 1000
	notification.LastEpochMicrosec = lastEpochMicrosec * 1000
	notification.NfNamingCode = nfNamingcode
	notification.NfcNamingCode = nfcNamingcode

	return notification
}
