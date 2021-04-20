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
package main

import (
	logging "VESPA/cimUtils"
	"VESPA/govel"
	"VESPA/logs"
	"VESPA/ves-agent/agent"
	"VESPA/ves-agent/config"
	"VESPA/ves-agent/configmapreader"
	"VESPA/ves-agent/zk"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// log "github.com/sirupsen/logrus"
)

// List of build information
var (
	version               = "dev"
	commit                = "none"
	date                  = "unknown"
	id                    []int64
	subj                  = "EVENT"
	firstFailed           = false
	returnFromFailedEvent = false
	host                  string
	input                 = ":"
	squareBraces          = "["
	replayOngoing         bool
	ReplayStruct          *govel.ReplayCollectorInfo
	rwmutex               = &sync.RWMutex{}
)

var doOnce sync.Once

func init() {
	// Initialize logging
	logs.InitializeLogInfo("ALL", true)
}

// launchVES launch the routine for
// - metric collection
// - heartbeat events
// - alert received events
func launchVES(ves govel.VESCollectorIf, conf *config.VESAgentConfiguration) {
	defer func() {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "launchVES", "launchVES", "VES routine exited")
	}()
	logging.LogForwarder("INFO", syscall.Gettid(), "launchVES", "launchVES", "Starting VES routine")
	logging.LogForwarder("INFO", syscall.Gettid(), "launchVES", "launchVES", "Started watching configmaps")

	configmapreader.GetServicePortCMaas()
	configmapreader.GetServicePortFMaas()
	agent := agent.NewAgent(conf)
	agent.StartAgent(conf.AlertManager.Bind, ves)
}

func launchVESWithoutUpdate(ves govel.VESCollectorIf, conf *config.VESAgentConfiguration) {
	defer func() {
		logging.LogForwarder("ERROR", syscall.Gettid(), "launchVES", "launchVES", "VES routine exited")
	}()
	logging.LogForwarder("INFO", syscall.Gettid(), "launchVES", "launchVES", "Starting VES routine")
	logging.LogForwarder("INFO", syscall.Gettid(), "launchVES", "launchVES", "Started watching configmaps")

	doOnce.Do(func() {
		go configmapreader.ConfigMapWatcher()

	})
	configmapreader.GetServicePortCMaas()
	configmapreader.GetServicePortFMaas()
	agent := agent.NewAgent(conf)
	agent.StartAgent(conf.AlertManager.Bind, ves)
}

func setInitReplay(w http.ResponseWriter, r *http.Request) {
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "setInitReplay", "Replay Initiated")
	reqBody, _ := ioutil.ReadAll(r.Body)
	fmt.Println("Replay request body from FMAAS", string(reqBody))

	fmt.Println("Replay r.body from FMAAS", r.Body)
	var replayFields govel.ReplayFields
	json.Unmarshal(reqBody, &replayFields)

	//setReplay Flag to TRUE
	// govel.ReplayCollectorInfo = govel.NewReplayObject(true, replayFields.CollectorID, govel.LastEventId, false)

	ReplayId := replayFields.CollectorID
	//ReplayStartIndex := replayFields.StartIndex
	if govel.CollectorFirstReplay[ReplayId] == false {
		replayOngoing = false
		govel.CollectorFirstReplay[ReplayId] = true
	} else {
		replayOngoing = true
	}
	ReplayStruct = govel.NewReplayObject(true, replayFields.CollectorID, govel.CollectorLastEventId[replayFields.CollectorID], replayOngoing, replayFields.StartIndex)

	govel.CollectorReplayObjects[ReplayId] = ReplayStruct

	fmt.Println("StartIndex from fmaas ..........>>", replayFields.StartIndex)
	resp := govel.ReplayResponse{
		CollectorStatus: logging.CollectorStatus[string(ReplayId)],
		ReplayOnGoing:   replayOngoing,
		LastSequenceID:  govel.CollectorLastEventId[ReplayId],
	}

	govel.ReplayChannel <- ReplayId

	w.WriteHeader(202)
	json.NewEncoder(w).Encode(resp) //send response back to FMAAS
	// w.WriteHeader(202)
}

func setResetReplay(w http.ResponseWriter, r *http.Request) {
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "setResetReplay", "ResetReplay Initiated")
	eventList := govel.UniqueEventList
	log.Println("event unique list", eventList)

	for _, uniqueId := range eventList {
		govel.SetResetReplay[uniqueId] = true
	}
	w.WriteHeader(202)

}

func initReplay() {
	//expose rest api for initReplay
	http.HandleFunc("/setInitReplay", setInitReplay)
	http.HandleFunc("/setResetReplay", setResetReplay)
	log.Fatal(http.ListenAndServe(":8095", nil))
}

func getContainerId() {
	out, err := exec.Command("/bin/bash", "-c", "/opt/bin/get_containerid.sh").Output()
	if err != nil {
		ts, _ := logs.Get(logs.LogConf.DateFormat, logs.LogConf.TimeFormat, logs.LogConf.TimeLocation)
		msg := "[" + ts + "][ERROR][" + strconv.Itoa(syscall.Gettid()) + "][getContainerId][exec.Command]" + " - " + "Error while getting the container id from get_containerid.sh. " + err.Error()
		logs.LogConf.PrintToFile(msg)
	}
	logs.LogConf.Info("Out is :", string(out))
	logging.ContainerID = strings.TrimSpace(string(out))

	logging.LogForwarder("INFO", syscall.Gettid(), "getContainerId", "exec.Command", "Container id for ves-gw cim :", logging.ContainerID)
}
func InitializeVesGwDefaultConf() {

	configMap, err := GetVesgwConfigMap()
	if err != nil {
		log.Println("Config Map read error", err.Error())
		os.Exit(1)
	}
	log.Println(configMap.Data["vesgw.json"])
	content := []byte(configMap.Data["vesgw.json"])
	fmt.Println("content from vegw.json ", string(content[:]))
	err = json.Unmarshal(content, &config.VesConfigOb)
	if err != nil {
		log.Println("Unmarshal of vesConfigObj err", err.Error())
		return
	}
}

// list out configmap in given namespace
func GetVesgwConfigMap() (*v1.ConfigMap, error) {
	client, _ := configmapreader.NewKubeConfig()

	configMapName := "vesgw-mgmt-cfg"
	//configMapName := "vesgw-v0.9.1-mgmt-cfg"
	fmt.Println("VESGW config map name", configMapName)

	Configmap, err := client.Client.CoreV1().ConfigMaps(os.Getenv("K8S_NAMESPACE")).Get(configMapName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return Configmap, nil
}
func fillCollectorDetails() {
	if config.VesConfigOb.Config.CollectorDetail == nil {
		return
	}

	collDtls := config.VesConfigOb.Config.CollectorDetail
	for i := range collDtls {
		fmt.Println("Collec details.........", collDtls)
		var clDt config.CollectorDetails
		clDt.Id = collDtls[i].Id

		exist := isValueInList(clDt.Id, govel.UniqueEventList)
		fmt.Println("Inside fillCollectorDetails.......", govel.UniqueEventList)
		if exist || len(govel.UniqueEventList) == 0 {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.DefaultConf.Domains.Fault.CollectorList", "Duplicate Collector Id: "+fmt.Sprintf("%d", clDt.Id))
		} else {
			govel.ConfigUpdate <- clDt.Id
		}

		if collDtls[i].PrimaryCollector != nil {
			clDt.PrimaryCollector = prepareCollDetails(collDtls[i].PrimaryCollector)
		}
		fmt.Println("Collec backup ", collDtls[i].BackupCollector)
		if collDtls[i].BackupCollector != nil {
			clDt.BackupCollector = prepareCollDetails(collDtls[i].BackupCollector)
		}

		config.DefaultConf.CollectorDetails = append(config.DefaultConf.CollectorDetails, &clDt)
	}
}
func prepareCollDetails(configCollDT *config.Collector) (clDt govel.CollectorConfiguration) {
	//clDt.ServerRoot = configCollDT.ServerRoot
	clDt.FQDN = configCollDT.Fqdn
	clDt.Port = configCollDT.Port
	clDt.Secure = configCollDT.Secure
	clDt.User = configCollDT.User
	clDt.Password = configCollDT.Password
	clDt.PassPhrase = configCollDT.Passphrase
	clDt.NbiType = configCollDT.NbiType
	clDt.Brokers = configCollDT.Brokers
	clDt.Topic = configCollDT.Topic
	clDt.KafkaTopic = configCollDT.KafkaTopic
	clDt.NbiFormat = configCollDT.NbiFormat
	clDt.HeartBeat = configCollDT.HeartBeat
	return clDt
}
func fillDomainDetails() {
	var dms config.DomainCollectors
	if config.VesConfigOb.Config.Domains == nil {
		return
	}
	if config.VesConfigOb.Config.Domains.Fault.CollectorLists != nil {
		dms.Fault = prepareDomainDetails(config.VesConfigOb.Config.Domains.Fault)
	}
	if config.VesConfigOb.Config.Domains.Tca.CollectorLists != nil {
		dms.Tca = prepareDomainDetails(config.VesConfigOb.Config.Domains.Tca)
	}
	if config.VesConfigOb.Config.Domains.Measurement.CollectorLists != nil {
		dms.Measurment = prepareDomainDetails(config.VesConfigOb.Config.Domains.Measurement)
	}
	if config.VesConfigOb.Config.Domains.Notification.CollectorLists != nil {
		dms.Notificaiton = prepareDomainDetails(config.VesConfigOb.Config.Domains.Notification)
	}

	config.DefaultConf.Domains = &dms
}
func prepareDomainDetails(configDms config.VESGWDomains) (clLst config.CollectorList) {

	collDomainsLst := configDms.CollectorLists

	for i := range collDomainsLst {
		var cllists config.CollectorId
		cllists.Id = collDomainsLst[i].Id
		clLst.CollectorId = append(clLst.CollectorId, cllists)
	}

	return clLst
}
func fillHeartbeatDetails() {
	config.DefaultConf.Heartbeat.DefaultInterval, _ = time.ParseDuration(config.VesConfigOb.Config.Heartbeat.DefaultInterval)
}
func fillMeasurementDetails() {
	config.DefaultConf.Measurement.DefaultInterval, _ = time.ParseDuration(config.VesConfigOb.Config.Measurement.DefaultInterval)
	config.DefaultConf.Measurement.MaxBufferingDuration, _ = time.ParseDuration(config.VesConfigOb.Config.Measurement.MaxBufferingDuration)
	var prometheus config.PrometheusConfig
	prometheus.Address = config.VesConfigOb.Config.Measurement.Prometheus.Address
	prometheus.Timeout, _ = time.ParseDuration(config.VesConfigOb.Config.Measurement.Prometheus.Timeout)
	prometheus.KeepAlive, _ = time.ParseDuration(config.VesConfigOb.Config.Measurement.Prometheus.Keepalive)

	config.DefaultConf.Measurement.Prometheus = prometheus
}
func fillEventDetails() {
	config.DefaultConf.Event.MaxSize = config.VesConfigOb.Config.Event.MaxSize
	config.DefaultConf.Event.RetryInterval, _ = time.ParseDuration(config.VesConfigOb.Config.Event.RetryInterval)
	config.DefaultConf.Event.MaxMissed = config.VesConfigOb.Config.Event.MaxMissed
}
func fillAlertManagerDetails() {
	config.DefaultConf.AlertManager.Bind = config.VesConfigOb.Config.AlertManager.Bind
}
func fillClusterDetails() {
	config.DefaultConf.Cluster.Debug = config.VesConfigOb.Config.Cluster.Debug
	config.DefaultConf.Cluster.DisplayLogs = config.VesConfigOb.Config.Cluster.DisplayLogs
}
func prepareVesgwConfigList(isUpdate bool) {

	var uniqueEventList []int64
	for _, faultDomainId := range config.DefaultConf.Domains.Fault.CollectorId {
		exist := isValueInList(faultDomainId.Id, uniqueEventList)
		if exist {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.DefaultConf.Domains.Fault.CollectorList", "Duplicate Collector Id: "+fmt.Sprintf("%d", faultDomainId.Id))
		} else {
			uniqueEventList = append(uniqueEventList, faultDomainId.Id)
			// govel.UniqueEventList = append(govel.UniqueEventList, faultDomainId.Id)
		}
	}

	for _, notificationDomainId := range config.DefaultConf.Domains.Notificaiton.CollectorId {
		exist := isValueInList(notificationDomainId.Id, uniqueEventList)
		if exist {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.DefaultConf.Domains.Notificaiton.CollectorList", "Duplicate Collector Id: "+fmt.Sprintf("%d", notificationDomainId.Id))
		} else {
			uniqueEventList = append(uniqueEventList, notificationDomainId.Id)
		}
	}

	for _, tcaDomainId := range config.DefaultConf.Domains.Tca.CollectorId {
		exist := isValueInList(tcaDomainId.Id, uniqueEventList)
		if exist {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.DefaultConf.Domains.Tca.CollectorList", "Duplicate Collector Id: "+fmt.Sprintf("%d", tcaDomainId.Id))
		} else {
			uniqueEventList = append(uniqueEventList, tcaDomainId.Id)
		}
	}
	govel.UniqueEventList = uniqueEventList
	var idList []int64
	// var collectorList []string
	for _, collector := range config.DefaultConf.CollectorDetails {
		exist := isValueInList(collector.Id, idList)
		if exist {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.DefaultConf.CollectorDetails", "Duplicate Collector Id: "+fmt.Sprintf("%d", collector.Id))
		} else {
			idList = append(idList, collector.Id)
		}

		// config.KeyStruct
		key := config.KeyStruct{
			collector.PrimaryCollector.FQDN,
			collector.PrimaryCollector.Port,
			collector.BackupCollector.FQDN,
			collector.BackupCollector.Port}

		urlP := collector.PrimaryCollector.FQDN + ":"
		CollectorURLP := fmt.Sprintf("%s%d", urlP, collector.PrimaryCollector.Port)
		CollectorURLPC := CollectorURLP + ":"
		CollectorURLPCWithId := fmt.Sprintf("%s%d", CollectorURLPC, collector.Id)

		govel.CollectorUrlToId[CollectorURLPCWithId] = CollectorURLP
		MapURLForPrimary := "http://user:pass@" + CollectorURLP + "/eventListener/v7"
		log.Println("Collector URL pushing from main", MapURLForPrimary)

		urlB := collector.BackupCollector.FQDN + ":"
		CollectorURLB := fmt.Sprintf("%s%d", urlB, collector.BackupCollector.Port)
		MapURLForBackUp := "http://user:pass@" + CollectorURLB + "/eventListener/v7"
		log.Println("Collector URL pushing from main", MapURLForBackUp)

		newRes := strings.Split(CollectorURLPCWithId, ":")
		collectorIDNew := newRes[2]

		for j, collectorUrl := range govel.CollectorList {
			resCreatingOld := strings.Split(collectorUrl, ":")
			if collectorIDNew == resCreatingOld[2] {
				govel.CollectorList = RemoveIndex(govel.CollectorList, j)
			}
		}

		existP := isCollectorInList(CollectorURLPCWithId, govel.CollectorList)
		if existP {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.DefaultConf.CollectorDetails", "Duplicate Collector FQDN: "+fmt.Sprintf("%d", collector.Id))
		} else {
			govel.CollectorList = append(govel.CollectorList, CollectorURLPCWithId)
		}

		govel.CollectorUrl[MapURLForPrimary] = collector.Id
		govel.CollectorUrl[MapURLForBackUp] = collector.Id
		logging.CollectorStatus[string(collector.Id)] = "UP"

		_, ok := config.VesCollectorObjectslist[key]
		if !ok {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.VesCollectorObjectslist", "Adding Collector "+" Primary Fqdn:port "+fmt.Sprintf("%s", collector.PrimaryCollector.FQDN)+" "+fmt.Sprintf("%s", collector.PrimaryCollector.Port)+" "+fmt.Sprintf("%s", collector.BackupCollector.FQDN)+" "+fmt.Sprintf("%s", collector.BackupCollector.Port))
			config.VesCollectorObjectslist[key] = collector.Id
		} else {
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.VesCollectorObjectslist", "Duplicate Collector Configuration found ", " Id: ", fmt.Sprintf("%d", collector.Id), " Primary Fqdn:port ", collector.PrimaryCollector.FQDN, fmt.Sprintf("%d", collector.PrimaryCollector.Port), " Backup Fqdn:Port ", collector.BackupCollector.FQDN, fmt.Sprintf("%d", collector.BackupCollector.Port))
		}
		govel.VesCollectorsHeartBeat[collector.Id] = collector.PrimaryCollector.HeartBeat
	}
	id = idList
	// govel.CollectorList = collectorList
	fmt.Println("CollectorList===================>>>>>>>>>>>>>>>>", govel.CollectorList)
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.VesCollectorObjectslist", "Starting VES Agent version "+fmt.Sprintf("%s", version))
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.VesCollectorObjectslist", fmt.Sprintf("Version=%s, Commit=%s, Date=%s, Go version=%s", version, commit, date, runtime.Version()))
	var err error

	if len(config.DefaultConf.CollectorDetails) != 0 {
		for _, collector := range config.DefaultConf.CollectorDetails {
			if !strings.Contains(collector.PrimaryCollector.FQDN, squareBraces) {
				if strings.Contains(collector.PrimaryCollector.FQDN, input) {
					collector.PrimaryCollector.FQDN = "[" + collector.PrimaryCollector.FQDN + "]"
				}
			}
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "collector.PrimaryCollector.FQDN", "collector.PrimaryCollector.FQDN in collector list "+fmt.Sprintf("%s", collector.PrimaryCollector.FQDN))
			if !strings.Contains(collector.BackupCollector.FQDN, squareBraces) {
				if strings.Contains(collector.BackupCollector.FQDN, input) {
					collector.BackupCollector.FQDN = "[" + collector.BackupCollector.FQDN + "]"
				}
			}
			logging.LogForwarder("INFO", syscall.Gettid(), "main", "collector.BackupCollector.FQDN", "collector.BackupCollector.FQDN in collector list "+fmt.Sprintf("%s", collector.BackupCollector.FQDN))
			fmt.Println("Event Configuration ....", config.DefaultConf.Event.MaxMissed)
			fmt.Println("Event Configuration ....", config.DefaultConf.Event.RetryInterval)
			govel.Ves, err = govel.NewCluster(&collector.PrimaryCollector, &collector.BackupCollector, config.DefaultConf.Event, config.DefaultConf.CaCert)

			if err != nil {
				logging.LogForwarder("EXCEPTION", syscall.Gettid(), "main", "govel.NewCluster", "Cannot initialize VES connection: "+err.Error())
			}
			fmt.Println("New Cluster conf details..", govel.Ves)
			fmt.Println("Collector Id..", collector.Id)
			rwmutex.Lock()
			govel.VesCollectorObjects[collector.Id] = govel.Ves
			rwmutex.Unlock()
			fmt.Println("govel.VesCollectorObjects ....", govel.VesCollectorObjects[collector.Id])

			// collectorobject := govel.CollectorObjectForKafka[collector.Id]
			govel.Collectorobject.CollectorObjectForNbiType = collector.PrimaryCollector.NbiType
			govel.Collectorobject.CollectorObjectForKafkaPrimary = collector.PrimaryCollector.Brokers
			govel.Collectorobject.CollectorObjectForKafkaBackup = collector.BackupCollector.Brokers
			govel.Collectorobject.CollectorObjectForPrimaryTopic = collector.PrimaryCollector.KafkaTopic
			govel.Collectorobject.CollectorObjectForBackupTopic = collector.BackupCollector.KafkaTopic
			govel.Collectorobject.CollectorObjectForNbiFormat = collector.PrimaryCollector.NbiFormat
			govel.CollectorObjectForKafka[collector.Id] = govel.Collectorobject
			fmt.Println("collectorobject for id", govel.Collectorobject, govel.Collectorobject.CollectorObjectForNbiFormat)
			rwmutex.Lock()
			govel.CollectorFirstReplay[collector.Id] = false
			govel.HeartbeatSent[collector.Id] = false
			rwmutex.Unlock()

			fmt.Println("Create path and set value in zk...")
			path := "/id/" + fmt.Sprintf("%d", collector.Id)
			exist := zk.PathExist(zk.ZKClient, path)
			if exist {
				val, _ := strconv.Atoi(string(zk.GetData(zk.ZKClient, path)))
				rwmutex.Lock()
				govel.ZKSequenceId[collector.Id] = int64(val)
				rwmutex.Unlock()
			} else {
				zk.Create(zk.ZKClient, path, []byte("0"))
				rwmutex.Lock()
				govel.ZKSequenceId[collector.Id] = 0
				rwmutex.Unlock()
			}

			offsetPath := "/offset/" + fmt.Sprintf("%d", collector.Id)
			offsetExist := zk.PathExist(zk.ZKClient, offsetPath)
			if offsetExist {
				val, _ := strconv.Atoi(string(zk.GetData(zk.ZKClient, offsetPath)))
				rwmutex.Lock()
				govel.ZKOffsetId[collector.Id] = int64(val)
				rwmutex.Unlock()
			} else {
				zk.Create(zk.ZKClient, offsetPath, []byte("0"))
				rwmutex.Lock()
				govel.ZKOffsetId[collector.Id] = 0
				rwmutex.Unlock()
			}
			// if isUpdate && hitCount > 1 {
			// 	go launchVES(govel.Ves, &config.DefaultConf)
			// } else {
			go launchVESWithoutUpdate(govel.Ves, &config.DefaultConf)
			//}
		}

	} else {
		logging.LogForwarder("WARNING", syscall.Gettid(), "main", "main", "No collector configured")
		m := http.NewServeMux()
		config.AlertMgrSrv = http.Server{Addr: config.DefaultConf.AlertManager.Bind, Handler: m}
		m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello, Collectors configuration NOT FOUND VES Gateway is running without any collector")
		})
		logging.LogForwarder("INFO", syscall.Gettid(), "main", "main", "Listening on port : "+fmt.Sprintf("%s", config.DefaultConf.AlertManager.Bind))
		ls_err := config.AlertMgrSrv.ListenAndServe()
		if ls_err != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "main", "http.ListenAndServe", ls_err.Error())
		}

	}

}

func main() {

	//var conf config.VESAgentConfiguration
	if err := config.InitConf(&config.DefaultConf); err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "main", "config.InitConf", "Cannot read config file :", err.Error())
	}
	//add Namespace
	//go agents.WatchConfigChange(connection.ConnectEtcd, "change-set/"+os.Getenv("K8S_NAMESPACE")+"/"+os.Getenv("K8S_POD_ID"))

	// Reading vesgw configuration from configmap
	InitializeVesGwDefaultConf()

	go initReplay()
	fmt.Println("Getting XGVelaID from vesgw configMap", config.DefaultConf.Heartbeat.AdditionalFields.XGVelaID)
	govel.VesgwXGVelaId = config.DefaultConf.Heartbeat.AdditionalFields.XGVelaID

	getContainerId()
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "config.InitConf", "Found config details : "+fmt.Sprintf("%v", config.DefaultConf))
	for {
		responseCode := xgvelaRegister()
		if responseCode == 200 {
			break
		}
		ts, _ := logs.Get(logs.LogConf.DateFormat, logs.LogConf.TimeFormat, logs.LogConf.TimeLocation)
		msg := "[" + ts + "][ERROR][" + strconv.Itoa(syscall.Gettid()) + "][main][xgvelaRegister]" + " - " + "Error while xgvela registration. Retrying... "
		logs.LogConf.PrintToFile(msg)
	}
	go func() {
		http.HandleFunc("/updateConfig", updateConfiguration)

		http.Handle("/metrics", promhttp.Handler())

		http.ListenAndServe(":8000", nil)
	}()
	logging.NatsConnection()
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "logging.NatsConnection", "Started ves gateway service ....")

	//address := config.DefaultConf.Measurement.Prometheus.Address
	//config.VesConfigOb
	fillCollectorDetails()

	fillDomainDetails()

	fillHeartbeatDetails()

	fillAlertManagerDetails()

	fillEventDetails()

	fillMeasurementDetails()

	fillClusterDetails()
	// logging.LogForwarder("INFO", syscall.Gettid(), "main", "main", "Started ves gateway service ....hots"+fmt.Sprintf("%s", host)+fmt.Sprintf("%s", port))
	zk.CreateClient()

	prepareVesgwConfigList(false)

	// address := config.DefaultConf.Measurement.Prometheus.Address
	// u, _ := url.Parse(address)
	// host, port, _ := net.SplitHostPort(u.Host)
	config.VesConfigOb.Config.Measurement.Prometheus.Address = config.DefaultConf.Measurement.Prometheus.Address
	go func() {
		for {
			if subj == "EVENT" {
				address := config.VesConfigOb.Config.Measurement.Prometheus.Address
				u, _ := url.Parse(address)
				host, port, _ := net.SplitHostPort(u.Host)
				logging.ConnCheck(subj, host, port)
				for _, collectorUrl := range govel.CollectorList {
					collectorUrlWithoutId := govel.CollectorUrlToId[collectorUrl]
					collectorHost, collectorPort, _ := net.SplitHostPort(collectorUrlWithoutId)
					res := strings.Split(collectorUrl, ":")
					collectorIDForConnection := res[2]
					logging.CollectorConnCheck(subj, collectorHost, collectorPort, collectorIDForConnection)
					time.Sleep(1 * time.Second)
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	zk.ZKClient.Close()
	logging.LogForwarder("INFO", syscall.Gettid(), "main", "main", "Stopping VES Agent version"+fmt.Sprintf("%s", version))

}

func updateConfiguration(w http.ResponseWriter, r *http.Request) {
	//fmt.Println("updating the configuration")
	//var updateConfig message
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	//var updateConfig interface{}
	var updateConfig = make(map[string]string)
	err = json.Unmarshal(body, &updateConfig)
	if err != nil {
		json.NewEncoder(w).Encode(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("Recieved the request change for config")

	config.ProcessUpdateConfig(updateConfig, logging.NatsConn)
	updateDefaultConfUponPatch(updateConfig["config-patch"])
	prepareVesgwConfigList(true)

}
func updateDefaultConfUponPatch(value string) {
	if strings.Contains(string(value[:]), "collectorDetails") {

		fillCollectorDetails()
	}

	if strings.Contains(string(value), "vesdomains") {
		fillDomainDetails()
	}

	if strings.Contains(string(value[:]), "heartbeat") {

		fillHeartbeatDetails()
	}

	if strings.Contains(string(value[:]), "measurement") {

		fillAlertManagerDetails()
	}

	if strings.Contains(string(value[:]), "event") {

		fillEventDetails()
	}

	if strings.Contains(string(value[:]), "alertManager") {

		fillMeasurementDetails()
	}

	if strings.Contains(string(value[:]), "cluster") {

		fillClusterDetails()
	}

	return
}

func isValueInList(value int64, list []int64) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func isCollectorInList(value string, list []string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func RemoveIndex(s []string, index int) []string {
	return append(s[:index], s[index+1:]...)
}

func xgvelaRegister() int {

	cimport := "6060"

	registerDetails := map[string]string{
		"container_name": "vesgw",
		"container_id":   os.Getenv("K8S_CONTAINER_ID"),
	}

	jsonValue, err := json.Marshal(registerDetails)
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "xgvelaRegister", "json.Marshal", "Error while data marshalling "+err.Error())
		return http.StatusBadRequest
	}

	response, err := http.Post("http://localhost:"+cimport+"/api/v1/_operations/xgvela/register", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "xgvelaRegister", "http.Post", "The HTTP request failed with error "+err.Error())
		return http.StatusInternalServerError
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "xgvelaRegister", "http.Post", "xgvela registration successfull "+fmt.Sprintf("%d", response.StatusCode))
	return response.StatusCode
}
