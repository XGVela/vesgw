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

package agent

import (
	logging "VESPA/cimUtils"
	"VESPA/govel"
	"VESPA/logs"
	"VESPA/ves-agent/config"
	"VESPA/ves-agent/convert"
	"VESPA/ves-agent/ha"
	"VESPA/ves-agent/heartbeat"
	"VESPA/ves-agent/metrics"
	"VESPA/ves-agent/rest"
	"VESPA/ves-agent/scheduler"
	"VESPA/ves-agent/zk"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	// log "github.com/sirupsen/logrus"
)

// Agent sends heartbeat, measurement and fault events.
// It initializes schedulers to trigger heartbeat and metric events.
// The schedulers are updated on new heartbeat and measurement interval.
// It initializes the AlertReceiver server to receive and handle Alert event from prometheus.
type Agent struct {
	measSched, hbSched           *scheduler.Scheduler
	measTimer, hbTimer           *time.Timer
	measIntervalCh, hbIntervalCh <-chan time.Duration
	alertCh                      chan rest.TcaFault
	fm                           *convert.FaultManager
	state                        *ha.Cluster
	namingCodes                  string
	stopScheduler                chan bool
}

var (
	//Fault is a fault event
	Fault    *govel.EventFault
	RCPFault *govel.RCPEventFault

	once sync.Once

	//TCA is a fault event
	TCA    *govel.EventTCA
	RCPTCA *govel.RCPEventTCA

	//Notification is a fault event
	Notification        *govel.EventNotification
	RCPNotification     *govel.RCPEventNotification
	replayCollectorInfo *govel.ReplayCollectorInfo
	rwmutex             = &sync.RWMutex{}
	ClusterState        *ha.Cluster
)

// NewAgent initializes schedulers to trigger heartbeat and metric events.
// It initializes the AlertReceiver server to receive Alert event from prometheus.
func NewAgent(conf *config.VESAgentConfiguration) *Agent {
	var err error
	conf.Cluster.ID = os.Getenv("HOSTNAME")
	fmt.Println("K8S_CONTAINER_ID: ", conf.Cluster.ID)
	rwmutex.Lock()
	ha.State, err = ha.NewCluster(conf.DataDir, conf.Cluster, ha.NewInMemState())
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "NewAgent", err.Error())
	}
	if ClusterState == nil && err == nil {
		fmt.Println("ClusterState nil updating clusterState")
		ClusterState = ha.State
	}
	rwmutex.Unlock()
	// create a FaultManager
	fm := convert.NewFaultManagerWithState(conf.Event, ClusterState)
	// declare the AlertReceiver route where the server will be listening

	return &Agent{

		fm: fm,

		state:       ClusterState,
		namingCodes: conf.Event.NfcNamingCodes,
	}
}

// Stats exposes some internal stats. This MUST be used ONLY
// for debugging purpose, and MUST NOT be considered stable
func (agent *Agent) Stats() map[string]interface{} {
	return map[string]interface{}{
		"raft": agent.state.Stats(),
	}
}

//func initMeasScheduler(conf *config.VESAgentConfiguration, namingCodes map[string]string, state ha.AgentState) *scheduler.Scheduler {
func initMeasScheduler(conf *config.NFMetricConfigRulesWithoutNfc, state ha.AgentState) *scheduler.Scheduler {
	// Creates a new measurements collector
	prom, err := metrics.NewCollectorWithState(&conf.Measurement, &conf.Event, state)
	if err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "agent", "initMeasScheduler", err.Error())
	}
	measSched := scheduler.NewSchedulerWithState("measurements", prom, conf.Measurement.DefaultInterval, state)
	return measSched
}

func initHbScheduler(conf *govel.EventConfigurationWithoutNfc, defaultInterval time.Duration, state ha.AgentState) *scheduler.Scheduler {
	// Creates a new heartbeat monitor
	hbMonitor, err := heartbeat.NewMonitorWithState(conf, state)
	if err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "agent", "initHbScheduler", err.Error())
	}
	hbSched := scheduler.NewSchedulerWithState("heartbeats", hbMonitor, defaultInterval, state)
	return hbSched
}

// initNfcNamingCode extract the vnfcNamingCode from vnfcName
func initNfcNamingCode(nfcNamingCodes []govel.NfcNamingCode) map[string]string {
	namingCodes := make(map[string]string)
	for _, nfcCode := range nfcNamingCodes {
		for _, vnfc := range nfcCode.Vnfcs {
			namingCodes[vnfc] = nfcCode.Type
		}
	}
	return namingCodes
}

// StartAgent registers to heartbeat and measurement interval changed events, and triggers the events.
// It initializes the AlertReceiver server to receive and handle Alert event from prometheus.
func (agent *Agent) StartAgent(bind string, ves govel.VESCollectorIf) {

	once.Do(func() {
		go agent.listen(bind, ves)
	})
	agent.serve(ves)
}

func (agent *Agent) listen(bind string, ves govel.VESCollectorIf) {
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "listen", "Setup measurement collection routine")

	// Subscribe to measurement interval changed events
	agent.measIntervalCh = ves.NotifyMeasurementIntervalChanged(make(chan time.Duration, 1024))

	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "listen", "Setup heartbeat routine for ves")
	// Subscribe to heartbeat interval changed events
	agent.hbIntervalCh = ves.NotifyHeartbeatIntervalChanged(make(chan time.Duration, 1024))

	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "listen", "Setup kafka event receiver server")

	// Setup the AlertReceiver and subscribe to alert events
	agent.notifyKafkaEventReceived(bind)
}

func (agent *Agent) notifyKafkaEventReceived(bind string) {
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "notifyKafkaEventReceived", "Staring consuming over kafka topics")

	// attach the AlertReceiver handler to the alert route managed by server
	agent.alertCh = make(chan rest.TcaFault, 1024)

	routes := []rest.Route{

		{Name: "Stats", Method: "GET", Pattern: "/stats", HandlerFunc: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			if err := enc.Encode(agent.Stats()); err != nil {
				ts, _ := logs.Get(logs.LogConf.DateFormat, logs.LogConf.TimeFormat, logs.LogConf.TimeLocation)
				msg := "[" + ts + "][ERROR][" + strconv.Itoa(syscall.Gettid()) + "][main][xgvelaRegister]" + " - " + "HTTP Handler - Cannot write stats : " + err.Error()
				logs.LogConf.PrintToFile(msg)
			}
		})},
	}
	var m sync.Mutex
	//start kafka consumer
	// agent.ConsumeMessages("FMAASEVENTS", 1) // FMAAS
	agent.ConsumeMessages("FMAASEVENTS", 1, &m)
	agent.ConsumeMessages("measurement", 1, &m)
	//new go routine measurement attribute
	// create an unstarted new server to receive http POST from prometheus
	alertHandler := rest.NewServer(routes)
	// logging.LogForwarder("INFO", syscall.Gettid(), "agent", "notifyKafkaEventReceived", "Starting server to receive http POST from prometheus")

	// start server
	go rest.StartServer(bind, alertHandler)

	for {
		select {
		case replayCollecorId := <-govel.ReplayChannel:
			// go func(id int64) {
			govel.PauseFmmasEvents[replayCollecorId] = true
			agent.ConsumeMessages("REPLAY", replayCollecorId, &m)
			// }(replayCollecorId)
		case appendedCollectorId := <-govel.ConfigUpdate:
			fmt.Println("Received NewCollectorId from ConfigUpdate Channel=========>>>>>>>>>", appendedCollectorId)
			agent.ConsumingOverFmaasEventToId(appendedCollectorId, "FMAASEVENTS", &m)
			agent.ConsumingOverMeasurementToId(appendedCollectorId, "measurement", &m)
		}
	}
}

func (agent *Agent) serve(ves govel.VESCollectorIf) {
	for {
		// Wait to become cluster's leader
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "serve", "Waiting to obtain cluster leadership")

		for !agent.followerStep() {
		}
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "serve", "Gained cluster leadership")

		// Setup schedulers timers

		// Run leadership steps until we loose leader state
		for agent.leaderStep(ves) {
		}
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "serve", "Lost cluster leadership")
		agent.measTimer.Stop()
		agent.hbTimer.Stop()
	}
}

func (agent *Agent) followerStep() bool {
	select {
	case fault := <-agent.alertCh:
		fault.Response <- errors.New("Not the leader")
		close(fault.Response)
	case leader := <-agent.state.LeaderCh():
		return leader
	}
	return false
}

func (agent *Agent) leaderStep(ves govel.VESCollectorIf) bool {
	// Demultiplex events
	select {

	case event := <-agent.alertCh:
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "leaderStep", "Received JSON from alertCh")

		//alert received events

		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "leaderStep", "Received event"+fmt.Sprintf("%v", event))
		agent.eventReceived(ves, event)

	case leader := <-agent.state.LeaderCh():
		return leader
	}
	return true
}

func (agent *Agent) handleMeasurementIntervalChanged(interval time.Duration) {
	if err := agent.measSched.SetInterval(interval); err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "handleMeasurementIntervalChanged", "Cannot update measurement interval : "+err.Error())
		return
	}
	agent.measTimer.Stop()
	agent.measTimer = agent.measSched.WaitChan()
}

func (agent *Agent) handleHeartbeatIntervalChanged(interval time.Duration) {
	if err := agent.hbSched.SetInterval(interval); err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "handleHeartbeatIntervalChanged", "Cannot update heartbeat interval : "+err.Error())
		return
	}
	agent.hbTimer.Stop()
	agent.hbTimer = agent.hbSched.WaitChan()
}

func (agent *Agent) triggerMeasurementEvent(ves *govel.Cluster) {
	triggerScheduler(agent.measSched, &agent.measTimer, func(res interface{}) error {
		b, err := json.Marshal(res.(metrics.EventMeasurementSet).Batch())
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "triggerMeasurementEvent", err.Error())
			logging.LogForwarder("EXCEPTION", syscall.Gettid(), "agent", "triggerMeasurementEvent", err.Error())
		}
		errfrompublish := publishToKafka(b)
		return errfrompublish
	})
}

func publishToKafka(meas []byte) error {
	writer := newKafkaWriter()
	defer writer.Close()

	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "publishToKafka", "start producing measurements to kafka !!", string(meas[:]))

	i := 0
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("Key-%d", i)),
		Value: meas,
	}
	err := writer.WriteMessages(context.Background(), msg)
	if err != nil {
		fmt.Println(err)
		return err
	}
	time.Sleep(1 * time.Second)
	return nil

}

func publishToKafkaForNbi(meas []byte, kafkaAddress []string, topic string, subj string) error {
	writer := newKafkaWriterForNbi(kafkaAddress, topic, subj)
	defer writer.Close()

	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "publishToKafkaForNbi", "start producing events to kafka !!", string(meas[:]))

	i := 0
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("Key-%d", i)),
		Value: meas,
	}
	err := writer.WriteMessages(context.Background(), msg)
	if err != nil {
		fmt.Println(err)
		return err
	}
	time.Sleep(1 * time.Second)
	return nil

}

func (agent *Agent) triggerHeatbeatEvent(ves *govel.Cluster) {
	triggerScheduler(agent.hbSched, &agent.hbTimer, func(res interface{}) error {
		b, err := json.Marshal(res.(govel.Event))
		if err != nil {
			log.Println(err)
		}
		errfrompublish := postHeartbeat(b)
		return errfrompublish

	})
}

func postHeartbeat(hb []byte) error {
	//PostEvent
	hmsg := govel.EventHeartbeatSet{}
	err := json.Unmarshal(hb, &hmsg)
	if err != nil {
		// log.Println("error while unmarshalling", err)
		return err
	}
	hbc := govel.NewHeartbeatCollector(hmsg.EventID, hmsg.EventName, hmsg.EventType, "VNFName", hmsg.HeartbeatInterval)
	hbc.SourceName = hmsg.SourceName
	hbc.NfNamingCode = hmsg.NfNamingCode
	//hbc.NfcNamingCode = hmsg.NfNamingCode
	hbc.ReportingEntityID = hmsg.ReportingEntityID
	hbc.ReportingEntityName = hmsg.ReportingEntityName
	hbc.SourceID = hmsg.SourceID
	hbc.NfVendorName = hmsg.NfVendorName
	hbc.AdditionalFields.CMAAS_NETCONF_PORT = govel.NetConfPort
	hbc.AdditionalFields.APIGW_PORT = govel.RestConfPort
	hbc.AdditionalFields.FMAAS_HTTP_PORT = govel.FmaasPort
	hbc.AdditionalFields.APIGW_USERNAME = "admin"
	hbc.AdditionalFields.APIGW_PASSWORD = "admin"
	hbc.AdditionalFields.APIGW_AUTH_TYPE = "basic"
	hbc.AdditionalFields.ME_ID = hmsg.AdditionalFields.ME_ID
	hbc.AdditionalFields.MGMT_IP = hmsg.AdditionalFields.MGMT_IP
	//eventList := govel.UniqueEventList
	for colId, hbState := range govel.VesCollectorsHeartBeat {
		// hmsg.AdditionalFields.COLLECTOR_ID = uniqueId
		if hbState {
			strId := strconv.FormatInt(colId, 10)
			hbc.AdditionalFields.COLLECTOR_ID = string(strId)
			vesObj := govel.VesCollectorObjects[colId]
			collectorobject := govel.CollectorObjectForKafka[colId]
			NbiTypeObject := collectorobject.CollectorObjectForNbiType
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat
			kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
			kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
			collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
			collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
			go func(vesObj *govel.Cluster, Id int64) {
				fmt.Println("heartbeatsent map before sending heartbeat===========>>>>>>>>>", govel.HeartbeatSent)
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postHeartbeat", "Sending Heartbeat for Collector with id: "+fmt.Sprintf("%s", string(Id)), " heartbeat: ", fmt.Sprintf("%v", hbc))

				if NbiTypeObject == "KAFKA" {
					reqBodyBytes := new(bytes.Buffer)
					json.NewEncoder(reqBodyBytes).Encode(hbc)
					if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
						if nbiformatObj == "RCP" {
							publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "RCP")
						} else {
							publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
						}
					} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
						if nbiformatObj == "RCP" {
							publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "RCP")
						} else {
							publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
						}
					}
					fmt.Println("First heartbeat sent for id HeartbeatSent===========>>>>>>>>>", govel.HeartbeatSent)
					rwmutex.Lock()
					govel.HeartbeatSent[Id] = true
					rwmutex.Unlock()
					fmt.Println("First heartbeat sent for after unlocking =====>>>>>")
				} else {
					err := vesObj.PostEvent(hbc, Id)
					if err != nil {
						rwmutex.Lock()
						govel.HeartbeatSent[Id] = false
						rwmutex.Unlock()
						logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "eventReceived", "Cannot post heartbeat: "+err.Error())
					} else {
						fmt.Println("First heartbeat sent for id HeartbeatSent===========>>>>>>>>>", govel.HeartbeatSent)
						rwmutex.Lock()
						govel.HeartbeatSent[Id] = true
						rwmutex.Unlock()
						fmt.Println("First heartbeat sent for after unlocking =====>>>>>")
					}
				}

			}(vesObj, colId)
		} else {
			fmt.Println("First heartbeat sent for id HeartbeatSent===========>>>>>>>>>", govel.HeartbeatSent)
			rwmutex.Lock()
			govel.HeartbeatSent[colId] = true
			rwmutex.Unlock()
			fmt.Println("First heartbeat sent for after unlocking =====>>>>>")
		}
	}
	return nil

}

func triggerScheduler(sched *scheduler.Scheduler, timer **time.Timer, f func(interface{}) error) {
	res, err := sched.Step()
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "triggerScheduler", "Cannot trigger scheduler", sched.Name(), err.Error())

		// Setup a retry timer
		*timer = time.NewTimer(10 * time.Second)
		return
	}
	if err = f(res); err == nil {
		// Acknowledge the scheduler interval(s) if send is successful
		if err := sched.Ack(); err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "triggerScheduler", "Cannot acknowledge scheduler execution :", err.Error())

			return
		}
		// Set timer to the next interval
		*timer = sched.WaitChan()
	} else {
		// If Post to active ves collector failed: setup a retry timer before trying to second ves collector
		*timer = time.NewTimer(10 * time.Second)
	}
}

func (agent *Agent) eventReceived(ves govel.VESCollectorIf, messageEvent rest.TcaFault) {

	if messageEvent.Alert.MavData.Domain == "fault" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Received domain fault through fault service")

		Fault = govel.NewFault(
			//	messageFault.Alert.MavData.Domain,
			//"fault",
			messageEvent.Alert.MavData.EventName,
			"xgvela",
			messageEvent.Alert.MavData.EventID,
			messageEvent.Alert.MavData.AlarmCondition,
			messageEvent.Alert.MavData.EventType,
			messageEvent.Alert.MavData.ReportingEntityID,
			messageEvent.Alert.MavData.ReportingEntityName,
			messageEvent.Alert.MavData.Sequence,
			messageEvent.Alert.MavData.SourceID,
			messageEvent.Alert.MavData.SpecificProblem,
			messageEvent.Alert.MavData.Priority,
			messageEvent.Alert.MavData.EventFaultSeverity,
			"Active",
			messageEvent.Alert.MavData.SourceName,
			messageEvent.Alert.MavData.LastEpochMillis,
			messageEvent.Alert.MavData.StartEpochMillis,
			messageEvent.Alert.MavData.EventCategory,
			messageEvent.Alert.MavData.EventSourceType,
			messageEvent.Alert.MavData.NfNamingCode,
			messageEvent.Alert.MavData.NfcNamingCode)

		Fault.AlarmAdditionalInformation = messageEvent.Alert.MavData.AlarmAdditionalInformation
		for _, faultDomainId := range config.DefaultConf.Domains.Fault.CollectorId {
			vesObj := govel.VesCollectorObjects[faultDomainId.Id]
			go func(vesObj *govel.Cluster, Id int64) {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Posting Event for Collector with id: ", fmt.Sprintf("%d", Id))

				if err := vesObj.PostEvent(Fault, Id); err != nil {
					logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "eventReceived", "Cannot post fault: "+err.Error())
				}
			}(vesObj, faultDomainId.Id)

		}
	} else if messageEvent.Alert.MavData.Domain == "notification" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Received Notification through fault service")

		Notification = govel.NewNotification(
			messageEvent.Alert.MavData.NotificationFields.ChangeContact,
			messageEvent.Alert.MavData.NotificationFields.ChangeIdentifier,
			messageEvent.Alert.MavData.NotificationFields.ChangeType,
			messageEvent.Alert.MavData.NotificationFields.NotificationFieldsVersion,
			messageEvent.Alert.MavData.NotificationFields.NewState,
			messageEvent.Alert.MavData.NotificationFields.OldState,
			messageEvent.Alert.MavData.NotificationFields.StateInterface,

			messageEvent.Alert.MavData.EventName,
			messageEvent.Alert.MavData.EventID,
			messageEvent.Alert.MavData.EventType,
			messageEvent.Alert.MavData.ReportingEntityID,
			messageEvent.Alert.MavData.ReportingEntityName,
			messageEvent.Alert.MavData.Sequence,
			messageEvent.Alert.MavData.SourceID,
			messageEvent.Alert.MavData.Priority,
			messageEvent.Alert.MavData.SourceName,
			"Active",
			messageEvent.Alert.MavData.LastEpochMillis,
			messageEvent.Alert.MavData.StartEpochMillis,
			messageEvent.Alert.MavData.NfNamingCode,
			messageEvent.Alert.MavData.NfcNamingCode)

		Notification.AdditionalFields = messageEvent.Alert.MavData.NotificationFields.AdditionalFields

		for _, notificationDomainId := range config.DefaultConf.Domains.Notificaiton.CollectorId {
			vesObj := govel.VesCollectorObjects[notificationDomainId.Id]
			go func(vesObj *govel.Cluster, Id int64) {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Posting Event for Collector with id: "+fmt.Sprintf("%d", Id))
				if err := vesObj.PostEvent(Notification, Id); err != nil {
					logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "eventReceived", "Cannot post fault: "+err.Error())
				}
			}(vesObj, notificationDomainId.Id)
		}
	} else if messageEvent.Alert.MavData.Domain == "thresholdCrossingAlert" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Received domain thresholdCrossingAlert through fault service")
		TCA = govel.NewTCA(

			messageEvent.Alert.MavData.AdditionalParameters,
			messageEvent.Alert.MavData.AlertAction,
			messageEvent.Alert.MavData.AlertDescription,
			messageEvent.Alert.MavData.AlertType,
			messageEvent.Alert.MavData.CollectionTimestamp,
			messageEvent.Alert.MavData.EventSeverity,
			messageEvent.Alert.MavData.EventStartTimestamp,
			messageEvent.Alert.MavData.ThresholdCrossingFieldsVersion,
			"TCA",
			"xgvela",
			messageEvent.Alert.MavData.EventID,
			messageEvent.Alert.MavData.EventName,
			messageEvent.Alert.MavData.EventType,
			messageEvent.Alert.MavData.ReportingEntityID,
			messageEvent.Alert.MavData.ReportingEntityName,
			messageEvent.Alert.MavData.Sequence,
			messageEvent.Alert.MavData.SourceID,
			messageEvent.Alert.MavData.Priority,
			"VirtualMachine",
			"Active",
			messageEvent.Alert.MavData.LastEpochMillis,
			messageEvent.Alert.MavData.StartEpochMillis,
			messageEvent.Alert.MavData.NfNamingCode,
			messageEvent.Alert.MavData.NfcNamingCode)

		TCA.AdditionalFields = messageEvent.Alert.MavData.AdditionalFields

		for _, tcaDomainId := range config.DefaultConf.Domains.Tca.CollectorId {
			vesObj := govel.VesCollectorObjects[tcaDomainId.Id]
			go func(vesObj *govel.Cluster, Id int64) {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Posting Event for Collector with id: "+fmt.Sprintf("%d", Id))
				if err := vesObj.PostEvent(TCA, Id); err != nil {
					logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "eventReceived", "Cannot post fault: "+err.Error())
				}
			}(vesObj, tcaDomainId.Id)
		}

	} else {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "eventReceived", "Json missing expected Domain")
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "eventReceived", "Closing alertmanager Response")
	close(messageEvent.Response)
}

func newKafkaWriter() *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  config.DefaultConf.KafkaAddress,
		Topic:    "measurement",
		Balancer: &kafka.LeastBytes{},
	})
}

func newKafkaWriterForNbi(kafkaAddress []string, topic string, subj string) *kafka.Writer {
	ka := ""
	for _, val := range kafkaAddress {
		ka += val + " "
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "newKafkaWriterForNbi", "KAFKA URL FOR RCP", ka)
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  kafkaAddress, //
		Topic:    topic,        // read from cm
		Balancer: &kafka.LeastBytes{},
	})
}

//GetKafkaReader reader object for measurement consumption
func GetKafkaReaderForMeas(topicName string, Id int64) *kafka.Reader {
	ka := ""
	for _, val := range config.DefaultConf.KafkaAddress {
		ka += val + " "
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "GetKafkaReaderForMeas", "kafka url :", ka, " for consuming over topic : ", topicName, " for collector id: "+fmt.Sprintf("%d", Id))
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.DefaultConf.KafkaAddress,
		Topic:    topicName,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		GroupID:  string(Id),
	})
}

//GetKafkaReader reader object for fmaas consumption
func GetKafkaReader(topicName string, Id int64) *kafka.Reader {
	ka := ""
	for _, val := range config.DefaultConf.KafkaAddress {
		ka += val + " "
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "GetKafkaReader", "KAFKA URL :", ka, "KAFKA TOPIC NAME :", topicName)

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.DefaultConf.KafkaAddress,
		Topic:    topicName,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		GroupID:  string(Id),
	})
}

//GetKafkaReader reader object for fmaas consumption
func GetKafkaReaderForResetReplay(topicName string, Id int64) *kafka.Reader {
	//log.Println("KAFKA URL", config.DefaultConf.KafkaAddress)
	//log.Println("KAFKA TOPIC NAME FOR READER", topicName)
	//	logging.LogForwarder("KAFKA TOPIC NAME" + fmt.Sprintf("%s", topicName))

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.DefaultConf.KafkaAddress,
		Topic:    topicName,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		GroupID:  string(Id),
		MaxWait:  30,
	})
}

func (agent *Agent) ConsumingOverFmaasEventToId(CollectorIDForFmaas int64, topicName string, m *sync.Mutex) {
	go func(Id int64) {

		for {
			rwmutex.Lock()
			if govel.HeartbeatSent[Id] {
				rwmutex.Unlock()
				break
			}
			rwmutex.Unlock()
			time.Sleep(1 * time.Second)
		}

		eventReader := GetKafkaReader(topicName, Id)
		log.Println("start consuming for topic: ", topicName)
		//logging.LogForwarder("start consuming for fmaas... !!" + fmt.Sprintf("%s", eventList))
		for {
			rwmutex.Lock()
			if govel.PauseFmmasEvents[Id] == false {
				rwmutex.Unlock()
				//rwmutex.Lock()
				//if govel.HeartbeatSent[Id] {
				//rwmutex.Unlock()
				message, err := eventReader.FetchMessage(context.Background())
				// message, err := eventReader.ReadMessage(context.Background())
				if err != nil {
					log.Fatalln(err)
				}
				rwmutex.Lock()
				govel.CollectorLastEventId[Id] = getLastSequenceId(message)
				rwmutex.Unlock()
				fmt.Printf("fmaas message at topic:%v partition:%v offset:%v	key:%v value:%v \n", message.Topic, message.Partition, message.Offset, message.Key, string(message.Value))
				//	logging.LogForwarder("fmaas message at topic:%v partition:%v offset:%v	key:%v value:%v \n" + fmt.Sprintf("%s", message.Topic) + fmt.Sprintf("%s", message.Partition) + fmt.Sprintf("%s", message.Offset) + fmt.Sprintf("%s", message.Key) + fmt.Sprintf("%s", string(message.Value)))
				fmt.Println("***********************************************")
				//logs.LogConf.Info("message at topic:" + message.Topic + " partition:" + string(message.Partition) + " offset:" + string(message.Offset) + "\n")

				log.Println("Inside else for customer check")
				rwmutex.Lock()
				if govel.PauseFmmasEvents[Id] == false {
					rwmutex.Unlock()

					agent.processMessages(message, topicName, Id)
					eventReader.CommitMessages(context.Background(), message)

				} else {
					rwmutex.Unlock()
					eventReader.CommitMessages(context.Background(), message)
					rwmutex.Lock()
					for govel.PauseFmmasEvents[Id] == true {
						rwmutex.Unlock()
						//fmt.Println("Replay is ongoing")
					}
					rwmutex.Unlock()
					//eventReader.CommitMessages(context.Background(), message)
					agent.processMessages(message, topicName, Id)

					//rwmutex.Unlock()
				}
				//} else {
				//	rwmutex.Unlock()
				//}

			} else {
				//for govel.PauseFmmasEvents[Id] == true {
				//	fmt.Println("Replay is ongoing")
				//}
				rwmutex.Unlock()
			}

		}
	}(CollectorIDForFmaas)
}

func (agent *Agent) ConsumingOverMeasurementToId(collectorIDForMeas int64, topicName string, m *sync.Mutex) {
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "ConsumeMessages", "Start consuming measurement from kafka for collector: "+fmt.Sprintf("%d", collectorIDForMeas))
	go func(Id int64) {
		reader := GetKafkaReaderForMeas(topicName, Id)
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "ConsumeMessages", "sart cosuming for collector id: "+fmt.Sprintf("%d", Id))

		for {
			message, err := reader.ReadMessage(context.Background())
			if err != nil {
				logging.LogForwarder("EXCEPTION", syscall.Gettid(), "agent", "ConsumeMessages", err.Error())
			}
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "ConsumeMessages", "message at topic : "+fmt.Sprintf("%s", message.Topic)+" partition : "+fmt.Sprintf("%d", message.Partition)+" offset :	"+fmt.Sprintf("%s", string(message.Offset))+" key : "+fmt.Sprintf("%d", string(message.Key))+" value : "+fmt.Sprintf("%s", string(message.Value)))
			agent.processMessagesForMeas(message, topicName, Id)
		}
	}(collectorIDForMeas)
}

//ConsumeMessages consume messages
func (agent *Agent) ConsumeMessages(topicName string, replayIdC int64, m *sync.Mutex) {
	// connect to kafka
	if topicName == "measurement" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "ConsumeMessages", "start consuming ... !!"+fmt.Sprintf("%s", topicName))

		for _, measurementDomainId := range config.DefaultConf.Domains.Measurment.CollectorId {
			//logging.LogForwarder("INFO", syscall.Gettid(), "agent", "ConsumeMessages", "Start consuming measurement from kafka for collector: "+fmt.Sprintf("%d", measurementDomainId.Id))
			agent.ConsumingOverMeasurementToId(measurementDomainId.Id, topicName, m)
		}

	} else if topicName == "FMAASEVENTS" {
		// connect to kafka
		eventList := govel.UniqueEventList
		log.Println("event unique list", eventList)
		//logging.LogForwarder("event unique list" + fmt.Sprintf("%s", eventList))
		for _, uniqueId := range eventList {
			log.Println("ID getting from fmaas for loop is....", uniqueId)
			agent.ConsumingOverFmaasEventToId(uniqueId, "FMAASEVENTS", m)
		}
	} else {
		uniqueId := replayIdC
		replayCollectorInfo = govel.CollectorReplayObjects[uniqueId]
		replayTopic := "FMAASEVENTS"
		agent.processEvents(replayTopic, replayCollectorInfo.ReplayCollectorId, replayCollectorInfo.StartIndex, m)
	}
}

func getLastOffset(Id int64) int64 {
	path := "/offset/" + fmt.Sprintf("%d", Id)
	exist := zk.PathExist(zk.ZKClient, path)
	if exist {
		val, _ := strconv.Atoi(string(zk.GetData(zk.ZKClient, path)))
		fmt.Println("LastOffset from zk...........>>> ", int64(val), string(val))
		return int64(val)

	} else {
		return 0

	}
}

func getLastSequence(Id int64) int64 {
	path := "/id/" + fmt.Sprintf("%d", Id)
	exist := zk.PathExist(zk.ZKClient, path)
	if exist {
		val, _ := strconv.Atoi(string(zk.GetData(zk.ZKClient, path)))
		fmt.Println("LastSequence from zk...........>>> ", int64(val), string(val))
		return int64(val)

	} else {
		return 0
	}
}

func (agent *Agent) processEvents(topicName string, Id int64, startIndex int64, m *sync.Mutex) {
	go func(topicName string, Id int64, m *sync.Mutex) {
		lastOffset := getLastOffset(Id)
		lastSequence := getLastSequence(Id)
		fmt.Println("Lastoffset, LastSequence for replay ...", lastOffset, lastSequence)
		startOffset := lastOffset - (lastSequence - startIndex)
		fmt.Println("StartOffset for replay ...", startOffset)
		eventReader := GetKafkaReaderForInitReplay(topicName, Id)
		eventReader.SetOffset(startOffset)
		for {

			message, err := eventReader.ReadMessage(context.Background())
			fmt.Println("Message from replay ...", message)
			if err != nil {
				log.Fatalln(err)
			}
			// replayCollectorInfo = govel.CollectorReplayObjects[Id]
			m.Lock()
			agent.processMessagesForReplay(message, topicName, Id)
			m.Unlock()
			if govel.SetResetReplay[Id] == true {
				govel.PauseFmmasEvents[Id] = false
				govel.CollectorFirstReplay[Id] = false
				govel.SetResetReplay[Id] = false
			} else {
				if govel.ZKSequenceId[Id] == govel.ZKSequenceIdForReplay[Id] {
					govel.PauseFmmasEvents[Id] = false
					govel.CollectorFirstReplay[Id] = false
					govel.SetResetReplay[Id] = false
				}

			}

		}
	}(topicName, Id, m)

}

func getLastSequenceId(msg kafka.Message) int64 {

	alertsmsg := govel.Data{}
	err := json.Unmarshal(msg.Value, &alertsmsg)
	if err != nil {
		log.Println("error while unmarshalling", err)
		//	logging.LogForwarder("error while unmarshalling" + err.Error())
		return 0
	}
	return alertsmsg.MavData.Sequence
}
func GetKafkaReaderForInitReplay(topicName string, Id int64) *kafka.Reader {
	log.Println("KAFKA URL FOR REPLAY", config.DefaultConf.KafkaAddress)
	log.Println("KAFKA TOPIC NAME FOR REPLAY READER", topicName)
	//logging.LogForwarder("KAFKA TOPIC NAME" + fmt.Sprintf("%s", topicName))

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  config.DefaultConf.KafkaAddress,
		Topic:    topicName,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
}

//processMessages process messages to ves
func (agent *Agent) processMessagesForMeas(msg kafka.Message, topicName string, Id int64) {

	// Non blocking write, to avoid a dead lock situation

	if topicName == "measurement" {

		//PostBatch
		measurementmsg := metrics.EventMeasurementSet{}
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessagesForMeas", "Start processing for measurement")

		err := json.Unmarshal(msg.Value, &measurementmsg)
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessagesForMeas", "error while unmarshalling"+err.Error())

			return
		}

		vesObj := govel.VesCollectorObjects[Id]

		collectorobject := govel.CollectorObjectForKafka[Id]
		NbiTypeObject := collectorobject.CollectorObjectForNbiType
		nbiformatObj := collectorobject.CollectorObjectForNbiFormat
		kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
		kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
		collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
		collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic

		if NbiTypeObject == "KAFKA" {
			reqBodyBytes := new(bytes.Buffer)
			json.NewEncoder(reqBodyBytes).Encode(measurementmsg.Batch())
			if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
				if nbiformatObj == "RCP" {
					publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "RCP")
				} else {
					publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
				}
			} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
				if nbiformatObj == "RCP" {
					publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "RCP")
				} else {
					publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
				}
			}
		} else {
			vesObj.PostBatch(measurementmsg.Batch(), Id)
		}

	}
}

func postEventForRCPFault(alertsmsg govel.RCPData, kafkaAddressPrimary []string, kafkaAddressBackup []string, PrimaryTopic string, BackupTopic string, Id int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceId[Id] + 1
	RCPFault = govel.NewFaultForRCP(
		zksequence,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.ReportingEntityName,
		alertsmsg.MavData.EventName,
		alertsmsg.MavData.EventFaultSeverity,
		alertsmsg.MavData.LocalEventName)
	//var alarmInput interface{}

	var rcpAdditionalfields govel.RCPEnhancedAdditionalFields
	fmt.Println("in faults alertsmsg.MavData.AlarmAdditionalInformation====>>>>", alertsmsg.MavData.AlarmAdditionalInformation)
	eventAdditionalFieldsbytes, _ := json.Marshal(alertsmsg.MavData.AlarmAdditionalInformation)
	json.Unmarshal(eventAdditionalFieldsbytes, &rcpAdditionalfields)
	fmt.Println("in faults rcpAdditionalfields after unmarshaling into this=========>>>", rcpAdditionalfields)
	RCPFault.Uhn = rcpAdditionalfields.Uhn
	RCPFault.Cnfc_uui = rcpAdditionalfields.Cnfc_uui
	RCPFault.Pod_uuid = rcpAdditionalfields.Pod_uuid
	if len(rcpAdditionalfields.CorrelationId) == 0 {
		RCPFault.Severity = "Info"
	} else {
		RCPFault.Severity = "Clear"
	}

	var faultBody govel.RCPFaultBody
	faultInputBodyByte, _ := json.Marshal(alertsmsg.MavData.RcpFaultFields)
	json.Unmarshal(faultInputBodyByte, &faultBody)
	faultBody.ProductType = os.Getenv("PRODUCT_TYPE")
	faultInputBodyByte, _ = json.Marshal(faultBody)
	faultInputfields := strings.Split(string(faultInputBodyByte[:]), "{")
	faultInputfields1 := strings.Split(faultInputfields[1], "}")
	faultBodyFields := faultInputfields1[0]

	alarmInput, _ := json.Marshal(alertsmsg.MavData.AlarmAdditionalInformation)
	alarmInputfields := strings.Split(string(alarmInput[:]), "{")
	alarmInputfields1 := strings.Split(alarmInputfields[1], "}")
	faultAlarmAdditionalFields := alarmInputfields1[0]

	rcpFaultBody := "{" + faultAlarmAdditionalFields + ", " + faultBodyFields + "}"
	fmt.Println(rcpFaultBody)
	jsonByte, _ := json.Marshal(rcpFaultBody)
	fmt.Println("Json Data", string(jsonByte[:]))
	json.Unmarshal([]byte(rcpFaultBody), &RCPFault.Body)

	//Publish To Kafka
	reqBodyBytes := new(bytes.Buffer)
	json.NewEncoder(reqBodyBytes).Encode(RCPFault)
	fmt.Println("For RCP fault before publish to kafka")
	fmt.Println("Zookeeper sequence: ", zksequence)
	fmt.Println("ReqBodyBytes======fault", string(reqBodyBytes.Bytes()))

	if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
		publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, PrimaryTopic, "RCP")
	} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
		publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, BackupTopic, "RCP")
	}
	govel.ZKSequenceId[Id]++
	path := "/id/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path, []byte(strconv.Itoa(int(govel.ZKSequenceId[Id]))))
	rwmutex.Unlock()
}

func postEventForRCPNotification(alertsmsg govel.RCPData, kafkaAddressPrimary []string, kafkaAddressBackup []string, PrimaryTopic string, BackupTopic string, Id int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceId[Id] + 1
	RCPNotification = govel.NewNotificationForRCP(
		zksequence,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.ReportingEntityName,
		alertsmsg.MavData.EventName,
		alertsmsg.MavData.Priority,
		alertsmsg.MavData.LocalEventName)

	var rcpAdditionalfields govel.RCPEnhancedAdditionalFields
	eventAdditionalFieldsbytes, _ := json.Marshal(alertsmsg.MavData.RcpNotificationFields.AdditionalFields)
	json.Unmarshal(eventAdditionalFieldsbytes, &rcpAdditionalfields)
	RCPNotification.Uhn = rcpAdditionalfields.Uhn
	RCPNotification.Cnfc_uui = rcpAdditionalfields.Cnfc_uui
	RCPNotification.Pod_uuid = rcpAdditionalfields.Pod_uuid

	var notificationBody govel.RCPEventBody
	notificationInputBodyByte, _ := json.Marshal(alertsmsg.MavData.RcpNotificationFields)
	json.Unmarshal(notificationInputBodyByte, &notificationBody)

	notificationBody.ProductType = os.Getenv("PRODUCT_TYPE")
	notificationInputBodyByte, _ = json.Marshal(notificationBody)
	notificationInputfields := strings.Split(string(notificationInputBodyByte[:]), "{")
	notificationInputfields1 := strings.Split(notificationInputfields[1], "}")
	notificationFields := notificationInputfields1[0]

	notificationEventInput, _ := json.Marshal(alertsmsg.MavData.RcpNotificationFields.AdditionalFields)
	notificationEventInputfields := strings.Split(string(notificationEventInput[:]), "{")
	notificationEventInputfields1 := strings.Split(notificationEventInputfields[1], "}")
	notificationAdditionalFields := notificationEventInputfields1[0]

	rcpNotificationBody := "{" + notificationAdditionalFields + ", " + notificationFields + "}"
	fmt.Println(rcpNotificationBody)
	jsonByte, _ := json.Marshal(rcpNotificationBody)
	fmt.Println("Json Data", string(jsonByte[:]))
	json.Unmarshal([]byte(rcpNotificationBody), &RCPNotification.Body)

	//Publish To Kafka
	reqBodyBytes := new(bytes.Buffer)
	json.NewEncoder(reqBodyBytes).Encode(RCPNotification)
	fmt.Println("For RCP notification before publish to kafka")
	fmt.Println("Zookeeper sequence: ", zksequence)
	fmt.Println("ReqBodyBytes======notification", string(reqBodyBytes.Bytes()))

	if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
		publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, PrimaryTopic, "RCP")
	} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
		publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, BackupTopic, "RCP")
	}
	govel.ZKSequenceId[Id]++
	path := "/id/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path, []byte(strconv.Itoa(int(govel.ZKSequenceId[Id]))))
	rwmutex.Unlock()
}

func isPrimaryBrokerReacheable(primary []string) bool {
	for _, kafkaAdd := range primary {
		timeout := 120 * time.Second
		_, err := net.DialTimeout("tcp", kafkaAdd, timeout)
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "isPrimaryBrokerReacheable", "Primary Kafka unreachable, error: ", err.Error())
			return false
		}
		return true
	}
	return false
}
func isSecondaryBrokerReacheable(secondary []string) bool {
	for _, kafkaAdd := range secondary {
		timeout := 120 * time.Second
		_, err := net.DialTimeout("tcp", kafkaAdd, timeout)
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "isSecondaryBrokerReacheable", "Backup Kafka unreachable, error: ", err.Error())
			return false
		}
		return true
	}
	return false
}
func postEventForRCPTCA(alertsmsg govel.RCPData, kafkaAddressPrimary []string, kafkaAddressBackup []string, PrimaryTopic string, BackupTopic string, Id int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceId[Id] + 1
	RCPTCA = govel.NewTCAForRCP(
		zksequence,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.ReportingEntityName,
		alertsmsg.MavData.EventName,
		alertsmsg.MavData.EventSeverity,
		alertsmsg.MavData.LocalEventName)

	var rcpAdditionalfields govel.RCPEnhancedAdditionalFields
	eventAdditionalFieldsbytes, _ := json.Marshal(alertsmsg.MavData.RcpThresholdCrossingAlertFields.AdditionalFields)
	json.Unmarshal(eventAdditionalFieldsbytes, &rcpAdditionalfields)

	fmt.Println("Atter unmarshalling to rcpAdditionalFIelds=======>>>>", rcpAdditionalfields)
	RCPTCA.Uhn = rcpAdditionalfields.Uhn
	RCPTCA.Cnfc_uui = rcpAdditionalfields.Cnfc_uui
	RCPTCA.Pod_uuid = rcpAdditionalfields.Pod_uuid
	var tcaBody govel.RCPTcaBody

	tcaInputBodyByte, _ := json.Marshal(alertsmsg.MavData.RcpThresholdCrossingAlertFields)
	json.Unmarshal(tcaInputBodyByte, &tcaBody)
	fmt.Println("Atter unmarshal=======>>>>", tcaBody)
	tcaBody.ProductType = os.Getenv("PRODUCT_TYPE")
	tcaInputBodyByte, _ = json.Marshal(tcaBody)

	tcaEventInput, _ := json.Marshal(alertsmsg.MavData.RcpThresholdCrossingAlertFields.AdditionalFields)
	fmt.Println("tcaEventInput after marshaling =======>>>>", string(tcaEventInput))
	tcaEventInputfields := strings.Split(string(tcaEventInput[:]), "{")
	tcaEventInputfields1 := strings.Split(tcaEventInputfields[1], "}")
	rcpTcaAdditonalFieldsBody := tcaEventInputfields1[0]

	rcpTCABody := "{" + rcpTcaAdditonalFieldsBody + "," + strings.Trim(string(tcaInputBodyByte), "{}") + "}"
	fmt.Println(rcpTCABody)
	jsonByte, _ := json.Marshal(rcpTCABody)
	fmt.Println("Json Data", string(jsonByte[:]))
	json.Unmarshal([]byte(rcpTCABody), &RCPTCA.Body)

	//Publish To Kafka
	reqBodyBytes := new(bytes.Buffer)
	json.NewEncoder(reqBodyBytes).Encode(RCPTCA)
	fmt.Println("For RCP TCA before publish to kafka")
	fmt.Println("Zookeeper sequence: ", zksequence)
	fmt.Println("ReqBodyBytes======tca", string(reqBodyBytes.Bytes()))

	if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
		publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, PrimaryTopic, "RCP")
	} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
		publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, BackupTopic, "RCP")
	}
	govel.ZKSequenceId[Id]++
	path := "/id/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path, []byte(strconv.Itoa(int(govel.ZKSequenceId[Id]))))
	rwmutex.Unlock()
}

func (agent *Agent) processMessagesForReplay(msg kafka.Message, topicName string, Id int64) {

	// Non blocking write, to avoid a dead lock situation
	fmt.Println("Coming inside processMessage")
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Replay in progress for FMAAS events")

	alertsmsg := govel.Data{}
	alertmsgRcp := govel.RCPData{}

	err := json.Unmarshal(msg.Value, &alertsmsg)
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessages", "error while unmarshalling"+err.Error())
		return
	}

	errrcp := json.Unmarshal(msg.Value, &alertmsgRcp)
	if errrcp != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessages", "error while unmarshalling"+err.Error())
		return
	}

	if alertsmsg.MavData.Domain == "fault" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Received domain fault through fault service")

		//Call Fault PostEvent
		//If Id present in configMap, then do postEvent
		_, found := Find("fault", Id)
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Fault ID found:", strconv.FormatBool(found))

		if found {
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Pushing Replay events to Endpoint ID :", fmt.Sprintf("%d", Id))
			collectorobject := govel.CollectorObjectForKafka[Id]
			//Check nbiFormat now
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat
			if nbiformatObj == "RCP" { // read from configmap, which customer

				// collectorKafkaAdressPrimary := govel.CollectorObjectForKafkaPrimary[Id]
				// collectorKafkaAdressBackup := govel.CollectorObjectForKafkaBackup[Id]
				// collectorPrimaryTopic := govel.CollectorObjectForPrimaryTopic[Id]
				// collectorBackupTopic := govel.CollectorObjectForBackupTopic[Id]

				// collectorobject := govel.CollectorObjectForKafka[Id]
				collectorKafkaAdressPrimary := collectorobject.CollectorObjectForKafkaPrimary
				collectorKafkaAdressBackup := collectorobject.CollectorObjectForKafkaBackup
				collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
				collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic

				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat ....!!", nbiformatObj, "start processing for RCP... !!")

				postEventForRCPFault(alertmsgRcp, collectorKafkaAdressPrimary, collectorKafkaAdressBackup, collectorPrimaryTopic, collectorBackupTopic, Id)
			} else {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat ....!!", nbiformatObj, "start processing for VES... !!")
				fmt.Println("Inside ProcessMessage for replay, checking offset of replay message.....", msg.Offset)
				postEventForFaultForReplay(alertsmsg, Id, msg.Offset)
			}
		}
	} else if alertsmsg.MavData.Domain == "notification" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Received domain notification through fault service")

		//Call Notification PostEvent
		//If Id present in configMap, then do postEvent
		_, found := Find("notification", Id)
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Notification ID found for Replay:", strconv.FormatBool(found))

		if found {
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Pushing Replay events to Endpoint ID :", fmt.Sprintf("%d", Id))
			collectorobject := govel.CollectorObjectForKafka[Id]
			//Check nbiFormat now
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat

			if nbiformatObj == "RCP" { // read from configmap, which customer
				// collectorKafkaAdressPrimary := govel.CollectorObjectForKafkaPrimary[Id]
				// collectorKafkaAdressBackup := govel.CollectorObjectForKafkaBackup[Id]
				// collectorPrimaryTopic := govel.CollectorObjectForPrimaryTopic[Id]
				// collectorBackupTopic := govel.CollectorObjectForBackupTopic[Id]
				// collectorobject := govel.CollectorObjectForKafka[Id]
				collectorKafkaAdressPrimary := collectorobject.CollectorObjectForKafkaPrimary
				collectorKafkaAdressBackup := collectorobject.CollectorObjectForKafkaBackup
				collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
				collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for Notification....!!", nbiformatObj, "start notification processing for RCP... !!")

				postEventForRCPNotification(alertmsgRcp, collectorKafkaAdressPrimary, collectorKafkaAdressBackup, collectorPrimaryTopic, collectorBackupTopic, Id)
			} else {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for Notification....!!", nbiformatObj, "start notification processing for VES... !!")
				fmt.Println("Inside ProcessMessage for replay, checking offset of replay message.....", msg.Offset)
				postEventForNotificationForReplay(alertsmsg, Id, msg.Offset)
			}
		}
	} else if alertsmsg.MavData.Domain == "thresholdCrossingAlert" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Received domain thresholdCrossingAlert through fault service")

		//Call TCA PostEvent
		//If Id present in configMap, then do postEvent
		_, found := Find("tca", Id)
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Tca ID found:", strconv.FormatBool(found))

		if found {
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Pushing events to Endpoint ID :", fmt.Sprintf("%d", Id))
			collectorobject := govel.CollectorObjectForKafka[Id]
			//Check nbiFormat now
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat
			if nbiformatObj == "RCP" { // read from configmap, which customer
				// collectorKafkaAdressPrimary := govel.CollectorObjectForKafkaPrimary[Id]
				// collectorKafkaAdressBackup := govel.CollectorObjectForKafkaBackup[Id]
				// collectorPrimaryTopic := govel.CollectorObjectForPrimaryTopic[Id]
				// collectorBackupTopic := govel.CollectorObjectForBackupTopic[Id]
				// collectorobject := govel.CollectorObjectForKafka[Id]
				collectorKafkaAdressPrimary := collectorobject.CollectorObjectForKafkaPrimary
				collectorKafkaAdressBackup := collectorobject.CollectorObjectForKafkaBackup
				collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
				collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for TCA....!!", nbiformatObj, "start notification processing for RCP... !!")

				postEventForRCPTCA(alertmsgRcp, collectorKafkaAdressPrimary, collectorKafkaAdressBackup, collectorPrimaryTopic, collectorBackupTopic, Id)
			} else {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for TCA....!!", nbiformatObj, "start tca processing for VES... !!")
				fmt.Println("Inside ProcessMessage for replay, checking offset of replay message.....", msg.Offset)
				postEventForTCAReplay(alertsmsg, Id, msg.Offset)
			}
		}
	} else {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessages", "Json missing expected Domain")
	}

	return
}

func (agent *Agent) processMessages(msg kafka.Message, topicName string, Id int64) {

	// Non blocking write, to avoid a dead lock situation
	fmt.Println("Coming inside processMessage")
	fmt.Println("Inside ProcessMessage, Offset of consumed message", msg.Offset)
	logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Coming inside process msg for FMAAS")

	alertsmsg := govel.Data{}
	alertmsgRcp := govel.RCPData{}

	err := json.Unmarshal(msg.Value, &alertsmsg)
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessages", "error while unmarshalling"+err.Error())
		return
	}

	errrcp := json.Unmarshal(msg.Value, &alertmsgRcp)
	if errrcp != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessages", "error while unmarshalling"+err.Error())
		return
	}

	if alertsmsg.MavData.Domain == "fault" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Received domain fault through fault service")

		//Call Fault PostEvent
		//If Id present in configMap, then do postEvent
		_, found := Find("fault", Id)
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Fault ID found:", strconv.FormatBool(found))

		if found {
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Pushing events to Endpoint ID :", fmt.Sprintf("%d", Id))
			collectorobject := govel.CollectorObjectForKafka[Id]
			//Check nbiFormat now
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat
			if nbiformatObj == "RCP" { // read from configmap, which customer

				// collectorKafkaAdressPrimary := govel.CollectorObjectForKafkaPrimary[Id]
				// collectorKafkaAdressBackup := govel.CollectorObjectForKafkaBackup[Id]
				// collectorPrimaryTopic := govel.CollectorObjectForPrimaryTopic[Id]
				// collectorBackupTopic := govel.CollectorObjectForBackupTopic[Id]

				// collectorobject := govel.CollectorObjectForKafka[Id]
				collectorKafkaAdressPrimary := collectorobject.CollectorObjectForKafkaPrimary
				collectorKafkaAdressBackup := collectorobject.CollectorObjectForKafkaBackup
				collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
				collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic

				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat ....!!", nbiformatObj, "start processing for RCP... !!")

				postEventForRCPFault(alertmsgRcp, collectorKafkaAdressPrimary, collectorKafkaAdressBackup, collectorPrimaryTopic, collectorBackupTopic, Id)
			} else {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat ....!!", nbiformatObj, "start processing for VES... !!")
				postEventForFault(alertsmsg, Id, msg.Offset)
			}
		}
	} else if alertsmsg.MavData.Domain == "notification" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Received domain notification through fault service")

		//Call Notification PostEvent
		//If Id present in configMap, then do postEvent
		_, found := Find("notification", Id)
		// logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Notification ID found:", strconv.FormatBool(found))

		if found {
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Pushing events to Endpoint ID :", fmt.Sprintf("%d", Id))
			collectorobject := govel.CollectorObjectForKafka[Id]
			//Check nbiFormat now
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat

			if nbiformatObj == "RCP" { // read from configmap, which customer
				// collectorKafkaAdressPrimary := govel.CollectorObjectForKafkaPrimary[Id]
				// collectorKafkaAdressBackup := govel.CollectorObjectForKafkaBackup[Id]
				// collectorPrimaryTopic := govel.CollectorObjectForPrimaryTopic[Id]
				// collectorBackupTopic := govel.CollectorObjectForBackupTopic[Id]
				// collectorobject := govel.CollectorObjectForKafka[Id]
				collectorKafkaAdressPrimary := collectorobject.CollectorObjectForKafkaPrimary
				collectorKafkaAdressBackup := collectorobject.CollectorObjectForKafkaBackup
				collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
				collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for Notification....!!", nbiformatObj, "start notification processing for RCP... !!")

				postEventForRCPNotification(alertmsgRcp, collectorKafkaAdressPrimary, collectorKafkaAdressBackup, collectorPrimaryTopic, collectorBackupTopic, Id)
			} else {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for Notification....!!", nbiformatObj, "start notification processing for VES... !!")

				postEventForNotification(alertsmsg, Id, msg.Offset)
			}
		}
	} else if alertsmsg.MavData.Domain == "thresholdCrossingAlert" {
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Received domain thresholdCrossingAlert through fault service")

		//Call TCA PostEvent
		//If Id present in configMap, then do postEvent
		_, found := Find("tca", Id)
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Tca ID found:", strconv.FormatBool(found))

		if found {
			logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Pushing events to Endpoint ID :", fmt.Sprintf("%d", Id))
			collectorobject := govel.CollectorObjectForKafka[Id]
			//Check nbiFormat now
			nbiformatObj := collectorobject.CollectorObjectForNbiFormat
			if nbiformatObj == "RCP" { // read from configmap, which customer
				// collectorKafkaAdressPrimary := govel.CollectorObjectForKafkaPrimary[Id]
				// collectorKafkaAdressBackup := govel.CollectorObjectForKafkaBackup[Id]
				// collectorPrimaryTopic := govel.CollectorObjectForPrimaryTopic[Id]
				// collectorBackupTopic := govel.CollectorObjectForBackupTopic[Id]
				// collectorobject := govel.CollectorObjectForKafka[Id]
				collectorKafkaAdressPrimary := collectorobject.CollectorObjectForKafkaPrimary
				collectorKafkaAdressBackup := collectorobject.CollectorObjectForKafkaBackup
				collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
				collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for TCA....!!", nbiformatObj, "start notification processing for RCP... !!")

				postEventForRCPTCA(alertmsgRcp, collectorKafkaAdressPrimary, collectorKafkaAdressBackup, collectorPrimaryTopic, collectorBackupTopic, Id)
			} else {
				logging.LogForwarder("INFO", syscall.Gettid(), "agent", "processMessages", "Checking the nbiFormat for TCA....!!", nbiformatObj, "start tca processing for VES... !!")

				postEventForTCA(alertsmsg, Id, msg.Offset)
			}
		}
	} else {
		logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "processMessages", "Json missing expected Domain")
	}

	return
}

func postEventForFault(alertsmsg govel.Data, Id int64, offset int64) {
	rwmutex.Lock()

	zksequence := govel.ZKSequenceId[Id] + 1
	Fault = govel.NewFault(

		alertsmsg.MavData.EventName,
		"xgvela",
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.AlarmCondition,
		alertsmsg.MavData.EventType,
		alertsmsg.MavData.ReportingEntityID,
		alertsmsg.MavData.ReportingEntityName,
		zksequence,
		alertsmsg.MavData.SourceID,
		alertsmsg.MavData.SpecificProblem,
		alertsmsg.MavData.Priority,
		alertsmsg.MavData.EventFaultSeverity,
		"Active",
		alertsmsg.MavData.SourceName,
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.EventCategory,
		alertsmsg.MavData.EventSourceType,
		alertsmsg.MavData.NfNamingCode,
		alertsmsg.MavData.NfcNamingCode)

	Fault.AlarmAdditionalInformation = alertsmsg.MavData.AlarmAdditionalInformation
	vesObj := govel.VesCollectorObjects[Id]
	fmt.Println("ves obj cluster for posting...", vesObj)
	fmt.Println("Zookeeper sequence: ", zksequence)
	fmt.Println("Fault in postEventForFault: ", Fault)

	collectorobject := govel.CollectorObjectForKafka[Id]
	NbiTypeObject := collectorobject.CollectorObjectForNbiType
	kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
	kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
	collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
	collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
	if NbiTypeObject == "KAFKA" {
		reqBodyBytes := new(bytes.Buffer)
		json.NewEncoder(reqBodyBytes).Encode(Fault)

		if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
		} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
		}

	} else {

		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postEventForFault", "Posting Event for Collector with id faultDomainId.Id: "+fmt.Sprintf("%d", Id), " Fault: ", fmt.Sprintf("%v", Fault))

		// if govel.HeartbeatSent[Id] {
		fmt.Println("HeartbeatSent map in postfault=======>>>>>>>>>>", govel.HeartbeatSent)
		if err := vesObj.PostEvent(Fault, Id); err != nil {
			// govel.CollectorStatus[Id] = "DOWN"
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "postEventForFault", "Cannot post fault: "+err.Error())

			// Send result to fault handler.
			//	messageFault.Response <- err
		} else {
			// govel.CollectorStatus[Id] = "UP"
		}
		// }
	}

	//	}
	govel.ZKSequenceId[Id]++
	path := "/id/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path, []byte(strconv.Itoa(int(govel.ZKSequenceId[Id]))))

	govel.ZKOffsetId[Id] = offset
	path_offset := "/offset/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path_offset, []byte(strconv.Itoa(int(offset))))

	rwmutex.Unlock()
}
func postEventForNotification(alertsmsg govel.Data, Id int64, offset int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceId[Id] + 1
	Notification = govel.NewNotification(
		alertsmsg.MavData.NotificationFields.ChangeContact,
		alertsmsg.MavData.NotificationFields.ChangeIdentifier,
		alertsmsg.MavData.NotificationFields.ChangeType,
		alertsmsg.MavData.NotificationFields.NotificationFieldsVersion,
		alertsmsg.MavData.NotificationFields.NewState,
		alertsmsg.MavData.NotificationFields.OldState,
		alertsmsg.MavData.NotificationFields.StateInterface,

		alertsmsg.MavData.EventName,
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.EventType,
		alertsmsg.MavData.ReportingEntityID,
		alertsmsg.MavData.ReportingEntityName,
		zksequence,
		alertsmsg.MavData.SourceID,
		alertsmsg.MavData.Priority,
		alertsmsg.MavData.SourceName,
		"Active",
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.NfNamingCode,
		alertsmsg.MavData.NfcNamingCode)

	Notification.AdditionalFields = alertsmsg.MavData.NotificationFields.AdditionalFields

	vesObj := govel.VesCollectorObjects[Id]

	fmt.Println("Zookeeper sequence: ", zksequence)
	fmt.Println("Notification in postEventForNotification: ", Notification)

	collectorobject := govel.CollectorObjectForKafka[Id]
	NbiTypeObject := collectorobject.CollectorObjectForNbiType
	kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
	kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
	collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
	collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
	if NbiTypeObject == "KAFKA" {
		reqBodyBytes := new(bytes.Buffer)
		json.NewEncoder(reqBodyBytes).Encode(Notification)

		if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
		} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
		}

	} else {

		//Check Id present in configmap
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postEventForNotification", "Posting Event for Collector with id: "+string(Id), " Notification: ", fmt.Sprintf("%v", Notification))

		// if govel.HeartbeatSent[Id] {
		fmt.Println("HeartbeatSent map in postNotification=======>>>>>>>>>>", govel.HeartbeatSent)
		if err := vesObj.PostEvent(Notification, Id); err != nil {
			// govel.CollectorStatus[Id] = "DOWN"
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "postEventForNotification", "Cannot post fault: "+err.Error())

			// Send result to fault handler.
			//messageFault.Response <- err
		} else {
			// govel.CollectorStatus[Id] = "UP"
		}
		// }
	}

	//	}
	govel.ZKSequenceId[Id]++
	path := "/id/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path, []byte(strconv.Itoa(int(govel.ZKSequenceId[Id]))))

	govel.ZKOffsetId[Id] = offset
	path_offset := "/offset/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path_offset, []byte(strconv.Itoa(int(offset))))

	rwmutex.Unlock()
}

func postEventForTCA(alertsmsg govel.Data, Id int64, offset int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceId[Id] + 1
	TCA = govel.NewTCA(

		alertsmsg.MavData.AdditionalParameters,
		alertsmsg.MavData.AlertAction,
		alertsmsg.MavData.AlertDescription,
		alertsmsg.MavData.AlertType,
		alertsmsg.MavData.CollectionTimestamp,
		alertsmsg.MavData.EventSeverity,
		alertsmsg.MavData.EventStartTimestamp,
		alertsmsg.MavData.ThresholdCrossingFieldsVersion,
		"TCA",
		"xgvela",
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.EventName,
		alertsmsg.MavData.EventType,
		alertsmsg.MavData.ReportingEntityID,
		alertsmsg.MavData.ReportingEntityName,
		zksequence,
		alertsmsg.MavData.SourceID,
		alertsmsg.MavData.Priority,
		"VirtualMachine",
		"Active",
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.NfNamingCode,
		alertsmsg.MavData.NfcNamingCode)

	TCA.AdditionalFields = alertsmsg.MavData.AdditionalFields

	vesObj := govel.VesCollectorObjects[Id]

	fmt.Println("Zookeeper sequence: ", zksequence)
	fmt.Println("TCA in postEventForTCA: ", TCA)

	collectorobject := govel.CollectorObjectForKafka[Id]
	NbiTypeObject := collectorobject.CollectorObjectForNbiType
	kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
	kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
	collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
	collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
	if NbiTypeObject == "KAFKA" {
		reqBodyBytes := new(bytes.Buffer)
		json.NewEncoder(reqBodyBytes).Encode(TCA)

		if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
		} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
		}

	} else {

		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postEventForTCA", "Posting Event for Collector with id: "+string(Id), " TCA: ", fmt.Sprintf("%v", TCA))

		// if govel.HeartbeatSent[Id] {
		fmt.Println("HeartbeatSent map in posttca=======>>>>>>>>>>", govel.HeartbeatSent)
		if err := vesObj.PostEvent(TCA, Id); err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "postEventForTCA", "Cannot post fault: ", err.Error())
			// govel.CollectorStatus[Id] = "DOWN"
			// Send result to fault handler.
			//	messageFault.Response <- err
		} else {
			// govel.CollectorStatus[Id] = "UP"
		}
		// }

	}
	//}
	govel.ZKSequenceId[Id]++
	path := "/id/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path, []byte(strconv.Itoa(int(govel.ZKSequenceId[Id]))))

	govel.ZKOffsetId[Id] = offset
	path_offset := "/offset/" + fmt.Sprintf("%d", Id)
	zk.Update(zk.ZKClient, path_offset, []byte(strconv.Itoa(int(offset))))

	rwmutex.Unlock()
}

//PostEvents For Replay
func postEventForFaultForReplay(alertsmsg govel.Data, Id int64, offset int64) {
	rwmutex.Lock()

	zksequence := govel.ZKSequenceIdForReplay[Id]
	Fault = govel.NewFault(

		alertsmsg.MavData.EventName,
		"xgvela",
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.AlarmCondition,
		alertsmsg.MavData.EventType,
		alertsmsg.MavData.ReportingEntityID,
		alertsmsg.MavData.ReportingEntityName,
		zksequence,
		alertsmsg.MavData.SourceID,
		alertsmsg.MavData.SpecificProblem,
		alertsmsg.MavData.Priority,
		alertsmsg.MavData.EventFaultSeverity,
		"Active",
		alertsmsg.MavData.SourceName,
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.EventCategory,
		alertsmsg.MavData.EventSourceType,
		alertsmsg.MavData.NfNamingCode,
		alertsmsg.MavData.NfcNamingCode)

	Fault.AlarmAdditionalInformation = alertsmsg.MavData.AlarmAdditionalInformation
	vesObj := govel.VesCollectorObjects[Id]
	fmt.Println("ves obj cluster for posting...", vesObj)

	collectorobject := govel.CollectorObjectForKafka[Id]
	NbiTypeObject := collectorobject.CollectorObjectForNbiType
	kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
	kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
	collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
	collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
	if NbiTypeObject == "KAFKA" {
		reqBodyBytes := new(bytes.Buffer)
		json.NewEncoder(reqBodyBytes).Encode(Fault)

		if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
		} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
		}

	} else {

		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postEventForFault", "Posting Event for Collector with id faultDomainId.Id: "+fmt.Sprintf("%d", Id), " Fault: ", fmt.Sprintf("%v", Fault))

		// if govel.HeartbeatSent[Id] {
		fmt.Println("HeartbeatSent map in postfault=======>>>>>>>>>>", govel.HeartbeatSent)
		if err := vesObj.PostEvent(Fault, Id); err != nil {
			// govel.CollectorStatus[Id] = "DOWN"
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "postEventForFault", "Cannot post fault: "+err.Error())

			// Send result to fault handler.
			//	messageFault.Response <- err
		} else {
			// govel.CollectorStatus[Id] = "UP"
		}
		// }
	}

	//	}
	govel.ZKSequenceIdForReplay[Id]++
	rwmutex.Unlock()
}
func postEventForNotificationForReplay(alertsmsg govel.Data, Id int64, offset int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceIdForReplay[Id]
	Notification = govel.NewNotification(
		alertsmsg.MavData.NotificationFields.ChangeContact,
		alertsmsg.MavData.NotificationFields.ChangeIdentifier,
		alertsmsg.MavData.NotificationFields.ChangeType,
		alertsmsg.MavData.NotificationFields.NotificationFieldsVersion,
		alertsmsg.MavData.NotificationFields.NewState,
		alertsmsg.MavData.NotificationFields.OldState,
		alertsmsg.MavData.NotificationFields.StateInterface,

		alertsmsg.MavData.EventName,
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.EventType,
		alertsmsg.MavData.ReportingEntityID,
		alertsmsg.MavData.ReportingEntityName,
		zksequence,
		alertsmsg.MavData.SourceID,
		alertsmsg.MavData.Priority,
		alertsmsg.MavData.SourceName,
		"Active",
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.NfNamingCode,
		alertsmsg.MavData.NfcNamingCode)

	Notification.AdditionalFields = alertsmsg.MavData.NotificationFields.AdditionalFields

	vesObj := govel.VesCollectorObjects[Id]

	collectorobject := govel.CollectorObjectForKafka[Id]
	NbiTypeObject := collectorobject.CollectorObjectForNbiType
	kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
	kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
	collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
	collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
	if NbiTypeObject == "KAFKA" {
		reqBodyBytes := new(bytes.Buffer)
		json.NewEncoder(reqBodyBytes).Encode(Notification)

		if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
		} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
		}

	} else {

		//Check Id present in configmap
		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postEventForNotification", "Posting Event for Collector with id: "+string(Id), " Notification: ", fmt.Sprintf("%v", Notification))

		// if govel.HeartbeatSent[Id] {
		fmt.Println("HeartbeatSent map in postNotification=======>>>>>>>>>>", govel.HeartbeatSent)
		if err := vesObj.PostEvent(Notification, Id); err != nil {
			// govel.CollectorStatus[Id] = "DOWN"
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "postEventForNotification", "Cannot post fault: "+err.Error())

			// Send result to fault handler.
			//messageFault.Response <- err
		} else {
			// govel.CollectorStatus[Id] = "UP"
		}
		// }
	}

	//	}
	govel.ZKSequenceIdForReplay[Id]++
	rwmutex.Unlock()
}

func postEventForTCAReplay(alertsmsg govel.Data, Id int64, offset int64) {
	rwmutex.Lock()
	zksequence := govel.ZKSequenceIdForReplay[Id]
	TCA = govel.NewTCA(

		alertsmsg.MavData.AdditionalParameters,
		alertsmsg.MavData.AlertAction,
		alertsmsg.MavData.AlertDescription,
		alertsmsg.MavData.AlertType,
		alertsmsg.MavData.CollectionTimestamp,
		alertsmsg.MavData.EventSeverity,
		alertsmsg.MavData.EventStartTimestamp,
		alertsmsg.MavData.ThresholdCrossingFieldsVersion,
		"TCA",
		"xgvela",
		alertsmsg.MavData.EventID,
		alertsmsg.MavData.EventName,
		alertsmsg.MavData.EventType,
		alertsmsg.MavData.ReportingEntityID,
		alertsmsg.MavData.ReportingEntityName,
		zksequence,
		alertsmsg.MavData.SourceID,
		alertsmsg.MavData.Priority,
		"VirtualMachine",
		"Active",
		alertsmsg.MavData.LastEpochMillis,
		alertsmsg.MavData.StartEpochMillis,
		alertsmsg.MavData.NfNamingCode,
		alertsmsg.MavData.NfcNamingCode)

	TCA.AdditionalFields = alertsmsg.MavData.AdditionalFields

	vesObj := govel.VesCollectorObjects[Id]

	collectorobject := govel.CollectorObjectForKafka[Id]
	NbiTypeObject := collectorobject.CollectorObjectForNbiType
	kafkaAddressPrimary := collectorobject.CollectorObjectForKafkaPrimary
	kafkaAddressBackup := collectorobject.CollectorObjectForKafkaBackup
	collectorPrimaryTopic := collectorobject.CollectorObjectForPrimaryTopic
	collectorBackupTopic := collectorobject.CollectorObjectForBackupTopic
	if NbiTypeObject == "KAFKA" {
		reqBodyBytes := new(bytes.Buffer)
		json.NewEncoder(reqBodyBytes).Encode(TCA)

		if isPrimaryBrokerReacheable(kafkaAddressPrimary) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressPrimary, collectorPrimaryTopic, "VESGW")
		} else if isSecondaryBrokerReacheable(kafkaAddressBackup) {
			publishToKafkaForNbi(reqBodyBytes.Bytes(), kafkaAddressBackup, collectorBackupTopic, "VESGW")
		}

	} else {

		logging.LogForwarder("INFO", syscall.Gettid(), "agent", "postEventForTCA", "Posting Event for Collector with id: "+string(Id), " TCA: ", fmt.Sprintf("%v", TCA))

		// if govel.HeartbeatSent[Id] {
		fmt.Println("HeartbeatSent map in posttca=======>>>>>>>>>>", govel.HeartbeatSent)
		if err := vesObj.PostEvent(TCA, Id); err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "agent", "postEventForTCA", "Cannot post fault: ", err.Error())
			// govel.CollectorStatus[Id] = "DOWN"
			// Send result to fault handler.
			//	messageFault.Response <- err
		} else {
			// govel.CollectorStatus[Id] = "UP"
		}
		// }

	}
	//}
	govel.ZKSequenceIdForReplay[Id]++

	rwmutex.Unlock()
}

//Check if ID is present in Domain Collector List, Return True or False
func Find(domain string, Id int64) (int, bool) {

	if domain == "fault" {
		for i, fmaasDomainId := range config.DefaultConf.Domains.Fault.CollectorId {
			if fmaasDomainId.Id == Id {
				return i, true
			}
		}
		return -1, false
	} else if domain == "notification" {
		for i, notificationDomainId := range config.DefaultConf.Domains.Notificaiton.CollectorId {
			if notificationDomainId.Id == Id {
				return i, true
			}
		}
		return -1, false
	} else if domain == "tca" {
		for i, tcaDomainId := range config.DefaultConf.Domains.Tca.CollectorId {
			if tcaDomainId.Id == Id {
				return i, true
			}
		}

	}
	return -1, false
}
