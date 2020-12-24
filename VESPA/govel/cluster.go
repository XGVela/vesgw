/*
	Copyright 2019 Nokia

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
	logging "VESPA/cimUtils"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
	// log "github.com/sirupsen/logrus"
)

// VESCollectorIf is the interface for the VES collector API
type VESCollectorIf interface {
	PostEvent(evt Event, Id int64) error
	PostBatch(batch Batch, Id int64) error
	GetMeasurementInterval() time.Duration
	GetHeartbeatInterval() time.Duration
	NotifyMeasurementIntervalChanged(ch chan time.Duration) <-chan time.Duration
	NotifyHeartbeatIntervalChanged(ch chan time.Duration) <-chan time.Duration
}

var Ves *Cluster
var SetResetFlag bool
var Collectorobject VesMap
var VesgwXGVelaId string
var idUsingUrl int64
var CollectorList []string

var (
	VesCollectorObjects     = make(map[int64]*Cluster)
	CollectorObjectForKafka = make(map[int64]VesMap)
	CollectorActiveFlag     = make(map[*Cluster]bool)
	CollectorReplayObjects  = make(map[int64]*ReplayCollectorInfo)
	CollectorFirstReplay    = make(map[int64]bool)
	UniqueEventList         []int64
	UniqueReplayList        []int64
	subj                    = "EVENT"
	CollectorLastEventId    = make(map[int64]int64)
	// CollectorStatus         = make(map[int64]string)
	CollectorUrl           = make(map[string]int64)
	CollectorUrlCheck      = make(map[string]int64)
	ReplayChannel          = make(chan int64)
	PauseFmmasEvents       = make(map[int64]bool)
	PreviousOffset         = make(map[int64]int64)
	CurrentOffset          = make(map[int64]int64)
	SetResetReplay         = make(map[int64]bool)
	CollectorUrlToId       = make(map[string]string)
	HeartbeatSent          = make(map[int64]bool)
	ConfigUpdate           = make(chan int64)
	ZKSequenceId           = make(map[int64]int64)
	ZKOffsetId             = make(map[int64]int64)
	ZKSequenceIdForReplay  = make(map[int64]int64)
	VesCollectorsHeartBeat = make(map[int64]bool)
)

type VesMap struct {
	CollectorObjectForNbiFormat    string
	CollectorObjectForNbiType      string
	CollectorObjectForKafkaPrimary []string
	CollectorObjectForKafkaBackup  []string
	CollectorObjectForPrimaryTopic string
	CollectorObjectForBackupTopic  string
}

// Cluster manage switches between collectors:
// primary and backup VES collector
// activ VES collector
type Cluster struct {
	activVES              *Evel
	primaryVES, backupVES *Evel
	maxMissed             int
	retryInterval         time.Duration
	mutex                 sync.RWMutex
}

func (cluster *Cluster) isPrimaryActive() bool {
	return cluster.activVES == cluster.primaryVES
}

// NewCluster initilizes the primary and backup ves collectors.
func NewCluster(prim *CollectorConfiguration, back *CollectorConfiguration, event *EventConfiguration, cacert string) (*Cluster, error) {
	primary, errP := NewEvel(prim, event, cacert)
	fmt.Println("Primary Coll from cluster creation", primary.baseURL)
	var backup *Evel
	var errB error
	if back.FQDN != "" {
		backup, errB = NewEvel(back, event, cacert)
		fmt.Println("Backup Coll from cluster creation", primary.baseURL)
	}
	activ := primary
	if errP != nil || primary == nil {
		if errB != nil || backup == nil {
			return &Cluster{}, errors.New("Cannot initialize any of the VES connection")
		}
		logging.LogForwarder("WARNING", syscall.Gettid(), "govel", "NewCluster", "Cannot initialize primary VES connection.")

		activ = backup
	} else {
		if errB != nil || backup == nil {
			logging.LogForwarder("WARNING", syscall.Gettid(), "govel", "NewCluster", "Cannot initialize backup VES connection.")

		}
	}
	return CreateCluster(activ, primary, backup, event.MaxMissed, event.RetryInterval)
}

// CreateCluster creates cluster from existing collectors.
func CreateCluster(activ, primary, backup *Evel, max int, retry time.Duration) (*Cluster, error) {
	return &Cluster{
		activVES:      activ,
		primaryVES:    primary,
		backupVES:     backup,
		maxMissed:     max,
		retryInterval: retry,
	}, nil
}

func NewReplayObject(initReplay bool, collectorId int64, lastSequence int64, replayOngoing bool, startSequence int64) *ReplayCollectorInfo {
	replayCollectorNew := new(ReplayCollectorInfo)
	replayCollectorNew.InitReplay = initReplay
	replayCollectorNew.LastEventId = lastSequence
	replayCollectorNew.ReplayCollectorId = collectorId
	replayCollectorNew.ReplayOnGoing = replayOngoing
	replayCollectorNew.StartIndex = startSequence
	// return &ReplayCollectorInfo{
	// 	InitReplay:        initReplay,
	// 	ReplayCollectorId: collectorId,
	// 	LastEventId:       lastSequence,
	// 	ReplayOnGoing:     replayOngoing,
	// }
	return replayCollectorNew
}

// GetMeasurementInterval returns the heartbeat measurement of the activ VES collector
// or 0 if agent's default interval should be used
func (cluster *Cluster) GetMeasurementInterval() time.Duration {
	cluster.mutex.RLock()
	defer cluster.mutex.RUnlock()
	return cluster.activVES.measurementInterval
}

// GetHeartbeatInterval returns the heartbeat interval of the activ VES collector
// or 0 if agent's default interval should be used
func (cluster *Cluster) GetHeartbeatInterval() time.Duration {
	cluster.mutex.RLock()
	defer cluster.mutex.RUnlock()
	return cluster.activVES.heartbeatInterval
}

// NotifyMeasurementIntervalChanged subscribe a channel to receive new measurement interval
// when it changes, from active and backup collector.
// The channel must be buffered or aggressively consumed.
// If the channel cannot be written, it won't receive events (writes are non blocking)
func (cluster *Cluster) NotifyMeasurementIntervalChanged(ch chan time.Duration) <-chan time.Duration {
	cluster.mutex.RLock()
	defer cluster.mutex.RUnlock()
	if cluster.activVES != nil {
		cluster.activVES.NotifyMeasurementIntervalChanged(ch)
	}
	if cluster.backupVES != nil {
		cluster.backupVES.NotifyMeasurementIntervalChanged(ch)
	}
	return ch
}

// NotifyHeartbeatIntervalChanged subscribe a channel to receive new heartbeat interval
// when it changes, from active and backup collector.
// The channel must be buffered or aggressively consumed.
// If the channel cannot be written, it won't receive events (writes are non blocking)
func (cluster *Cluster) NotifyHeartbeatIntervalChanged(ch chan time.Duration) <-chan time.Duration {
	cluster.mutex.RLock()
	defer cluster.mutex.RUnlock()
	if cluster.activVES != nil {
		cluster.activVES.NotifyHeartbeatIntervalChanged(ch)
	}
	if cluster.backupVES != nil {
		cluster.backupVES.NotifyHeartbeatIntervalChanged(ch)
	}
	return ch
}

// PostEvent sends an event to the activ VES collector
func (cluster *Cluster) PostEvent(evt Event, id int64) error {
	return cluster.perform("event", id, func(ves *Evel) error { return ves.PostEvent(evt) })
}

// PostBatch sends a list of events to VES collector in a single
// request using the batch interface
func (cluster *Cluster) PostBatch(batch Batch, id int64) error {
	return cluster.perform("batch", id, func(ves *Evel) error { return ves.PostBatch(batch) })
}

func (cluster *Cluster) perform(info string, id int64, f func(ves *Evel) error) error {
	fmt.Println("Cluster Details...", cluster.activVES)
	fmt.Println("Cluster Details...", cluster.backupVES)
	fmt.Println("Cluster Details...", cluster.mutex)
	fmt.Println("Cluster Details...", cluster.activVES.baseURL.String())
	cluster.mutex.RLock()
	defer cluster.mutex.RUnlock()
	var err error
	for nbRetry := 0; nbRetry <= cluster.maxMissed; nbRetry++ {

		if err = f(VesCollectorObjects[id].activVES); err != nil {
			fmt.Println("error from perform ", err.Error())
			logging.LogForwarder("ERROR", syscall.Gettid(), "govel", "perform", "Cannot initialize primary VES connection.")

			logging.LogForwarder("INFO", syscall.Gettid(), "govel", "perform", "Retry No. "+fmt.Sprintf("%d", nbRetry))

			if nbRetry == cluster.maxMissed {
				logging.LogForwarder("ERROR", syscall.Gettid(), "govel", "perform", "VES collector unreachable, switch.")
				idUsingUrl = CollectorUrl[cluster.activVES.baseURL.String()]

				// if CollectorStatus[idUsingUrl] == "UP" {
				// 	CollectorStatus[idUsingUrl] = "DOWN"
				// 	// logging.ConnCheckForSimulator(subj, cluster.activVES.baseURL.String(), "CollectorDown")
				// }
				cluster.switchCollector()
				nbRetry = 0
			} else {
				// idUsingUrl := CollectorUrl[cluster.activVES.baseURL.String()]
				// CollectorStatus[idUsingUrl] = "DOWN"
				logging.LogForwarder("INFO", syscall.Gettid(), "govel", "perform", "Retry post "+fmt.Sprintf("%s", info)+"in "+fmt.Sprintf("%s", cluster.retryInterval.String()))

				time.Sleep(cluster.retryInterval)
			}
		} else {
			fmt.Println("Pushing event")
			logging.LogForwarder("DEBUG", syscall.Gettid(), "govel", "perform", "Post succesfull "+fmt.Sprintf("%s", info))

			return nil
		}
	}
	return err
}

// SwitchCollector switch the activ VES server
func (cluster *Cluster) switchCollector() {
	if cluster.isPrimaryActive() || cluster.primaryVES == nil {
		if cluster.backupVES != nil {
			cluster.activVES = cluster.backupVES
			logging.LogForwarder("DEBUG", syscall.Gettid(), "govel", "switchCollector", "Use backup collector.")
		} else {
			logging.LogForwarder("DEBUG", syscall.Gettid(), "govel", "switchCollector", "No backup collector stay on primary.")
		}
	} else {
		cluster.activVES = cluster.primaryVES
		logging.LogForwarder("DEBUG", syscall.Gettid(), "govel", "switchCollector", "Use primary collector.")

	}
}
