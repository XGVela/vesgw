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
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	mdcloggo "gerrit.o-ran-sc.org/r/com/golog.git"
)

var appmgrDomain string

const appmgrXAppConfigPath = "/ric/v1/config"
const appmgrPort = "8080"

// VesMgr contains runtime information of the vesmgr process
type VesMgr struct {
	myIPAddress         string
	chXAppSubscriptions chan subscriptionNotification
	chXAppNotifications chan []byte
	chSupervision       chan chan string
	chVesagent          chan error
	vesagent            cmdRunner
	httpServer          HTTPServer
}

type subscriptionNotification struct {
	subscribed bool
	err        error
	subsID     string
}

var logger *mdcloggo.MdcLogger

// Version information, which is filled during compilation
// Version tag of vesmgr container
var Version string

// Hash of the git commit used in building
var Hash string

const vesmgrXappNotifPort = "8080"
const vesmgrXappNotifPath = "/vesmgr_xappnotif/"
const timeoutPostXAppSubscriptions = 5
const vespaConfigFile = "/etc/ves-agent/ves-agent.yaml"

func init() {
	logger, _ = mdcloggo.InitLogger("vesmgr")
}

func getMyIP() (myIP string, retErr error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Error("net.InterfaceAddrs failed: %s", err.Error())
		return "", err
	}
	for _, addr := range addrs {
		// check the address type and if it is not a loopback take it
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				logger.Info("My IP Address: %s", ipnet.IP.String())
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", nil
}

func createConf(fname string, xappMetrics []byte) {
	f, err := os.Create(fname)
	if err != nil {
		logger.Error("Cannot create vespa conf file: %s", err.Error())
		os.Exit(1)
	}
	defer f.Close()

	createVespaConfig(f, xappMetrics)
	logger.Info("Vespa config created")
}

func (vesmgr *VesMgr) subscribeXAppNotifications() {
	xappNotifURL := "http://" + vesmgr.myIPAddress + ":" + vesmgrXappNotifPort + vesmgrXappNotifPath
	subsURL := "http://" + appmgrDomain + ":" + appmgrPort + appmgrSubsPath
	go subscribexAppNotifications(xappNotifURL, vesmgr.chXAppSubscriptions, timeoutPostXAppSubscriptions, subsURL)
	logger.Info("xApp notifications subscribed from %s", subsURL)
}

// Init initializes the vesmgr
func (vesmgr *VesMgr) Init(listenPort string) *VesMgr {
	logger.Info("vesmgrInit")
	logger.Info("version %s (%s)", Version, Hash)

	var err error
	if vesmgr.myIPAddress, err = getMyIP(); err != nil || vesmgr.myIPAddress == "" {
		logger.Error("Cannot get myIPAddress: IP %s, err %s", vesmgr.myIPAddress, err.Error())
		panic("Cannot get my IP address")
	}

	var ok bool
	appmgrDomain, ok = os.LookupEnv("VESMGR_APPMGRDOMAIN")
	if ok {
		logger.Info("Using appmgrdomain %s", appmgrDomain)
	} else {
		appmgrDomain = "service-ricplt-appmgr-http.ricplt.svc.cluster.local"
		logger.Info("Using default appmgrdomain %s", appmgrDomain)
	}
	vesmgr.chXAppSubscriptions = make(chan subscriptionNotification)
	// Create notifications as buffered channel so that
	// xappmgr does not block if we are stuck somewhere
	vesmgr.chXAppNotifications = make(chan []byte, 10)
	vesmgr.chSupervision = make(chan chan string)
	vesmgr.chVesagent = make(chan error)
	vesmgr.httpServer = HTTPServer{}
	vesmgr.httpServer.init(vesmgr.myIPAddress + ":" + listenPort)
	vesmgr.vesagent = makeRunner("ves-agent", "-i", os.Getenv("VESMGR_HB_INTERVAL"),
		"-m", os.Getenv("VESMGR_MEAS_INTERVAL"), "--Measurement.Prometheus.Address",
		os.Getenv("VESMGR_PROMETHEUS_ADDR"))
	return vesmgr
}

func (vesmgr *VesMgr) startVesagent() {
	vesmgr.vesagent.run(vesmgr.chVesagent)
}

func (vesmgr *VesMgr) killVespa() error {
	logger.Info("Killing vespa")
	err := vesmgr.vesagent.kill()
	if err != nil {
		logger.Error("Cannot kill vespa: %s", err.Error())
		return err
	}
	return <-vesmgr.chVesagent // wait vespa exit
}

func queryXAppsConfig(appmgrURL string, timeout time.Duration) ([]byte, error) {
	emptyConfig := []byte("{}")
	logger.Info("query xAppConfig started, url %s", appmgrURL)
	req, err := http.NewRequest("GET", appmgrURL, nil)
	if err != nil {
		logger.Error("Failed to create a HTTP request: %s", err)
		return emptyConfig, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	client.Timeout = time.Second * timeout
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Query xApp config failed: %s", err)
		return emptyConfig, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Failed to read xApp config body: %s", err)
			return emptyConfig, err
		}
		logger.Info("query xAppConfig completed")
		return body, nil
	}
	logger.Error("Error from xApp config query: %s", resp.Status)
	return emptyConfig, errors.New(resp.Status)
}

func queryConf() ([]byte, error) {
	return queryXAppsConfig("http://"+appmgrDomain+":"+appmgrPort+appmgrXAppConfigPath,
		10*time.Second)
}

func (vesmgr *VesMgr) emptyNotificationsChannel() {
	for {
		select {
		case <-vesmgr.chXAppNotifications:
			// we don't care the content
		default:
			return
		}
	}
}

func (vesmgr *VesMgr) servRequest() {
	select {
	case supervision := <-vesmgr.chSupervision:
		logger.Info("vesmgr: supervision")
		supervision <- "OK"
	case xAppNotif := <-vesmgr.chXAppNotifications:
		logger.Info("vesmgr: xApp notification")
		logger.Info(string(xAppNotif))
		vesmgr.emptyNotificationsChannel()
		/*
		* If xapp config query fails then we cannot create
		* a new configuration and kill vespa.
		* In that case we assume that
		* the situation is fixed when the next
		* xapp notif comes
		 */
		xappConfig, err := queryConf()
		if err == nil {
			vesmgr.killVespa()
			createConf(vespaConfigFile, xappConfig)
			vesmgr.startVesagent()
		}
	case err := <-vesmgr.chVesagent:
		logger.Error("Vesagent exited: " + err.Error())
		os.Exit(1)
	}
}

func (vesmgr *VesMgr) waitSubscriptionLoop() {
	for {
		select {
		case supervision := <-vesmgr.chSupervision:
			logger.Info("vesmgr: supervision")
			supervision <- "OK"
		case isSubscribed := <-vesmgr.chXAppSubscriptions:
			if isSubscribed.err != nil {
				logger.Error("Failed to make xApp subscriptions, vesmgr exiting: %s", isSubscribed.err)
				os.Exit(1)
			}
			return
		}
	}
}

// Run the vesmgr process main loop
func (vesmgr *VesMgr) Run() {
	logger.Info("vesmgr main loop ready")
	vesmgr.httpServer.start(vesmgrXappNotifPath, vesmgr.chXAppNotifications, vesmgr.chSupervision)
	vesmgr.subscribeXAppNotifications()
	vesmgr.waitSubscriptionLoop()
	xappConfig, _ := queryConf()
	createConf(vespaConfigFile, xappConfig)
	vesmgr.startVesagent()
	for {
		vesmgr.servRequest()
	}
}
