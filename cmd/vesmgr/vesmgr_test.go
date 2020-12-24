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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestGetMyIP(t *testing.T) {
	myIPAddress, err := getMyIP()
	assert.NotEqual(t, string(""), myIPAddress)
	assert.Nil(t, err)
}

func TestConfCreate(t *testing.T) {
	tmpfile := filepath.Join(os.TempDir(), "vestest."+strconv.Itoa(os.Getpid()))
	defer os.Remove(tmpfile) // clean up
	createConf(tmpfile, []byte("{}"))
	_, err := os.Stat(tmpfile)
	assert.Nil(t, err)
}

type VesmgrTestSuite struct {
	suite.Suite
	vesmgr VesMgr
}

func (suite *VesmgrTestSuite) SetupSuite() {
	suite.vesmgr = VesMgr{}
	suite.vesmgr.Init("0")
	logger.MdcAdd("Testvesmgr", "0.0.1")
	os.Setenv("VESMGR_HB_INTERVAL", "30s")
	os.Setenv("VESMGR_MEAS_INTERVAL", "30s")
	os.Setenv("VESMGR_PRICOLLECTOR_ADDR", "127.1.1.1")
	os.Setenv("VESMGR_PRICOLLECTOR_PORT", "8443")
	os.Setenv("VESMGR_PROMETHEUS_ADDR", "http://localhost:9090")
}

func (suite *VesmgrTestSuite) TestMainLoopSupervision() {
	go suite.vesmgr.servRequest()
	ch := make(chan string)
	suite.vesmgr.chSupervision <- ch
	reply := <-ch
	suite.Equal("OK", reply)
}

func (suite *VesmgrTestSuite) TestMainLoopVesagentError() {
	if os.Getenv("TEST_VESPA_EXIT") == "1" {
		// we're run in a new process, now make vesmgr main loop exit
		go suite.vesmgr.servRequest()
		suite.vesmgr.chVesagent <- errors.New("vesagent killed")
		// we should never actually end up to this sleep, since the runVesmgr should exit
		time.Sleep(3 * time.Second)
		return
	}

	// Run the vesmgr exit test as a separate process
	cmd := exec.Command(os.Args[0], "-test.run", "TestVesMgrSuite", "-testify.m", "TestMainLoopVesagentError")
	cmd.Env = append(os.Environ(), "TEST_VESPA_EXIT=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	// check that vesmgr existed with status 1

	e, ok := err.(*exec.ExitError)
	suite.True(ok)
	suite.Equal("exit status 1", e.Error())
}

func (suite *VesmgrTestSuite) TestWaitSubscriptionLoopRespondsSupervisionAndBreaksWhenReceivedSubsNotif() {
	go func() {
		time.Sleep(time.Second)
		ch := make(chan string)
		suite.vesmgr.chSupervision <- ch
		suite.Equal("OK", <-ch)
		suite.vesmgr.chSupervision <- ch
		suite.Equal("OK", <-ch)
		suite.vesmgr.chXAppSubscriptions <- subscriptionNotification{true, nil, ""}
	}()

	suite.vesmgr.waitSubscriptionLoop()
}

func (suite *VesmgrTestSuite) TestEmptyNotificationChannelReadsAllMsgsFromCh() {
	go func() {
		for i := 0; i < 11; i++ {
			suite.vesmgr.chXAppNotifications <- []byte("hello")
		}
	}()
	time.Sleep(500 * time.Millisecond)
	<-suite.vesmgr.chXAppNotifications
	suite.vesmgr.emptyNotificationsChannel()
	select {
	case <-suite.vesmgr.chXAppNotifications:
		suite.Fail("Got unexpected notification")
	default:
		// ok
	}
}

func (suite *VesmgrTestSuite) TestVespaKilling() {
	suite.vesmgr.vesagent = makeRunner("sleep", "20")
	suite.vesmgr.startVesagent()
	suite.NotNil(suite.vesmgr.killVespa())
}

func (suite *VesmgrTestSuite) TestVespaKillingAlreadyKilled() {
	suite.vesmgr.vesagent = makeRunner("sleep", "20")
	suite.vesmgr.startVesagent()
	suite.NotNil(suite.vesmgr.killVespa())
	// Just check that second kill does not block execution
	suite.NotNil(suite.vesmgr.killVespa())
}

func TestVesMgrSuite(t *testing.T) {
	suite.Run(t, new(VesmgrTestSuite))
}
