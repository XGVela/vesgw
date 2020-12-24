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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AppmgrHTTPServerTestSuite struct {
	suite.Suite
	subscriptions chan subscriptionNotification
	xappNotifURL  string
}

// suite setup
func (suite *AppmgrHTTPServerTestSuite) SetupSuite() {
	// the url here is not actually used anywhere
	suite.xappNotifURL = "http://127.0.0.1:8080" + vesmgrXappNotifPath
	suite.subscriptions = make(chan subscriptionNotification)
}

// test setup
func (suite *AppmgrHTTPServerTestSuite) SetupTest() {
	suite.subscriptions = make(chan subscriptionNotification)
}

func (suite *AppmgrHTTPServerTestSuite) TestSubscribexAppNotifications() {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		var result map[string]interface{}
		err := json.Unmarshal([]byte(body), &result)
		suite.Nil(err)
		suite.Equal(5, int(result["maxRetries"].(float64)))
		suite.Equal(5, int(result["retryTimer"].(float64)))
		suite.Equal("all", result["eventType"].(string))
		suite.Equal("POST", req.Method)
		res.Header().Add("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)
		res.Write([]byte(`{"id":"deadbeef1234567890", "version":0, "eventType":"all"}`))
	}))
	defer testServer.Close()

	go subscribexAppNotifications(suite.xappNotifURL, suite.subscriptions, 1, testServer.URL)
	isSubscribed := <-suite.subscriptions
	suite.Nil(isSubscribed.err)
	suite.Equal("deadbeef1234567890", isSubscribed.subsID)
}

func (suite *AppmgrHTTPServerTestSuite) TestSubscribexAppNotificationsWrongStatus() {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Content-Type", "application/json")
		res.WriteHeader(http.StatusUnauthorized)
		res.Write([]byte(`{"id":"deadbeef1234567890", "version":0, "eventType":"all"}`))
	}))
	defer testServer.Close()

	requestBody := []byte(fmt.Sprintf(`{"maxRetries": 5, "retryTimer": 5, "eventType":"all", "targetUrl": "%v"}`, suite.xappNotifURL))
	req, _ := http.NewRequest("POST", testServer.URL, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	subsID, err := subscribexAppNotificationsClientDo(req, client)
	suite.Equal(errWrongStatusCode, err)
	// after failed POST vesmgr.appmgrSubsId holds an initial values
	suite.Equal("", subsID)
}

func (suite *AppmgrHTTPServerTestSuite) TestSubscribexAppNotificationsWrongUrl() {
	// use fake appmgrUrl that is not served in unit test
	appmgrURL := "/I_do_not_exist/"
	requestBody := []byte(fmt.Sprintf(`{"maxRetries": 5, "retryTimer": 5, "eventType":"all", "targetUrl": "%v"}`, suite.xappNotifURL))
	req, _ := http.NewRequest("POST", appmgrURL, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	subsID, err := subscribexAppNotificationsClientDo(req, client)
	suite.Equal(errPostingFailed, err)
	// after failed POST vesmgr.appmgrSubsId holds an initial values
	suite.Equal("", subsID)
}

func (suite *AppmgrHTTPServerTestSuite) TestSubscribexAppNotificationsReadBodyFails() {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Length", "1")
		res.Header().Add("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)
	}))
	defer testServer.Close()

	go subscribexAppNotifications(suite.xappNotifURL, suite.subscriptions, 1, testServer.URL)
	isSubscribed := <-suite.subscriptions
	suite.Equal("unexpected EOF", isSubscribed.err.Error())
	suite.Equal("", isSubscribed.subsID)
}

func (suite *AppmgrHTTPServerTestSuite) TestSubscribexAppNotificationsUnMarshalFails() {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Content-Type", "application/json")
		res.WriteHeader(http.StatusCreated)
		res.Write([]byte(`{""dump for UT": make(chan int),"}`))
	}))
	defer testServer.Close()

	go subscribexAppNotifications(suite.xappNotifURL, suite.subscriptions, 1, testServer.URL)
	isSubscribed := <-suite.subscriptions
	suite.Equal("invalid character 'd' after object key", isSubscribed.err.Error())
	suite.Equal("", isSubscribed.subsID)
}

func TestAppmgrHttpServerTestSuite(t *testing.T) {
	suite.Run(t, new(AppmgrHTTPServerTestSuite))
}
