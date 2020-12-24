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
	"VESPA/ves-agent/config"
	"fmt"
	"io/ioutil"
	"syscall"

	// "log"
	"sync"

	"gopkg.in/yaml.v2"
)

var (
	mu             sync.Mutex
	scheduledFiles = make(map[string]*Agent)
	ves            *govel.Cluster
)

const (
	Modified = "MODIFIED"
	Added    = "ADDED"
	Init     = "INITIAL"
)

//StartNfAgentProcessing nf wise processing
func StartNfAgentProcessing(action string, nfFileName string) {
	files, err := ioutil.ReadDir("./nfs")
	if err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "nf_agent", "StartNfAgentProcessing", err.Error())
	}

	for _, file := range files {
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "StartNfAgentProcessing", "processing file:"+fmt.Sprintf("%s", file.Name()))

		// var conf *config.NFMetricConfigRules
		var conf *config.NFMetricConfigRulesWithoutNfc
		fileName := "./nfs/" + file.Name()
		if action != Init && fileName != nfFileName {
			continue
		}
		//read file
		yamlFile, err := ioutil.ReadFile(fileName)
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "nf_agent", "StartNfAgentProcessing", "yamlFile.Get err  "+err.Error())

			return
		}

		err = yaml.Unmarshal(yamlFile, &conf)
		if err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "nf_agent", "StartNfAgentProcessing", "Unmarshal: "+err.Error())
		}
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "StartNfAgentProcessing", "reading nf wise conf are:"+fmt.Sprintf("%v", conf))

		if action == Modified && fileName == nfFileName {
			StopScheduler(fileName)
			go startSreaming(fileName, conf)
			return
		}

		if action == Added && fileName == nfFileName {
			go startSreaming(fileName, conf)
			return
		}
		go startSreaming(fileName, conf)

	}
}

func startSreaming(fName string, conf *config.NFMetricConfigRulesWithoutNfc) {
	agentCfg, measurementRequired := newNfAgent(conf)
	// It's time to collect and send some measurements
	if measurementRequired {
		for _, measurementDomainId := range config.DefaultConf.Domains.Measurment.CollectorId {
			// if measurementDomainId.Id == collector.Id {
			ves = govel.VesCollectorObjects[measurementDomainId.Id]
			go func(ves *govel.Cluster) {
				agentCfg.serveNF(ves, fName, measurementRequired)
			}(ves)
		}
	} else {
		fmt.Println("Measurement not configured, Starting heartbeat ", measurementRequired)
		go func(ves *govel.Cluster) {
			agentCfg.serveNF(ves, fName, measurementRequired)
		}(ves)
	}
}

//StopScheduler stop scheduler for a nf
func StopScheduler(filename string) {
	mu.Lock()
	schdCfg, ok := scheduledFiles[filename]
	mu.Unlock()
	if ok {
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "StopScheduler", "nf "+filename+" deleted. Stopping the scheduler......")

		schdCfg.stopScheduler <- true
	}
}

//newNfAgent initializes schedulers to trigger heartbeat and metric events.
func newNfAgent(conf *config.NFMetricConfigRulesWithoutNfc) (*Agent, bool) {
	if len(conf.Measurement.Prometheus.Rules) != 0 {
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "newNfAgent", "Create measurement scheduler")

		// Create a new Scheduler used to trigger the measurements collector
		measSched := initMeasScheduler(conf, ClusterState)

		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "newNfAgent", "Create heartbeat scheduler")

		// Create a new Scheduler used to trigger the heartbeat events
		hbSched := initHbScheduler(&conf.Event, config.DefaultConf.Heartbeat.DefaultInterval, ClusterState)

		return &Agent{
			measSched:     measSched,
			hbSched:       hbSched,
			state:         ClusterState,
			namingCodes:   conf.Event.NfNamingCode,
			stopScheduler: make(chan bool),
		}, true
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "newNfAgent", "Create heartbeat scheduler")
	// Create a new Scheduler used to trigger the heartbeat events
	hbSched := initHbScheduler(&conf.Event, config.DefaultConf.Heartbeat.DefaultInterval, ClusterState)
	return &Agent{

		hbSched:       hbSched,
		state:         ClusterState,
		namingCodes:   conf.Event.NfNamingCode,
		stopScheduler: make(chan bool),
	}, false
}

func (agent *Agent) serveNF(ves *govel.Cluster, fName string, measurementReq bool) {
	for {
		// Setup schedulers timers
		if measurementReq {
			agent.measTimer = agent.measSched.WaitChan()
		}
		agent.hbTimer = agent.hbSched.WaitChan()
		mu.Lock()
		//store filename and respective scheduler object
		scheduledFiles[fName] = agent
		mu.Unlock()
		// Run leadership steps until we loose leader state
		for agent.leaderNfStep(ves, measurementReq) {
		}
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "serveNF", "Lost cluster leadership")

		if measurementReq {
			agent.measTimer.Stop()
		}
		agent.hbTimer.Stop()
	}
}

func (agent *Agent) leaderNfStep(ves *govel.Cluster, measurementReq bool) bool {
	// Demultiplex events
	if measurementReq {
		select {
		case <-agent.measTimer.C:
			// It's time to collect and send some measurements
			agent.triggerMeasurementEvent(ves)
			logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "leaderNfStep", "Triggering Measurement.......")
		}

	}
	select {
	case measInterval := <-agent.measIntervalCh:
		// Measurements collection interval changed event
		agent.handleMeasurementIntervalChanged(measInterval)
	case hbInterval := <-agent.hbIntervalCh:
		// Heartbeat interval changed event
		agent.handleHeartbeatIntervalChanged(hbInterval)
	case <-agent.hbTimer.C:
		// It's time to send the heartbeat
		agent.triggerHeatbeatEvent(ves)
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "leaderNfStep", "Triggering heartbeat.......")

	case <-agent.stopScheduler:
		//Its time to stop the scheduler
		if measurementReq {
			agent.measTimer.Stop()
		}
		agent.hbTimer.Stop()
		logging.LogForwarder("INFO", syscall.Gettid(), "nf_agent", "leaderNfStep", "stopped scheduler.......")
	}
	return true
}
