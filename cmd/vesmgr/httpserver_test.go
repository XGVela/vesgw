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
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type HTTPServerTestSuite struct {
	suite.Suite
	chNotif       chan []byte
	chSupervision chan chan string
	server        HTTPServer
}

// suite setup creates the HTTP server
func (suite *HTTPServerTestSuite) SetupSuite() {
	os.Unsetenv("http_proxy")
	os.Unsetenv("HTTP_PROXY")
	suite.chNotif = make(chan []byte)
	suite.chSupervision = make(chan chan string)
	suite.server = HTTPServer{}
	suite.server.init(":0")
	suite.server.start("/vesmgr_notif/", suite.chNotif, suite.chSupervision)
}

func (suite *HTTPServerTestSuite) TestHtppServerSupervisionInvalidOperation() {
	resp, reply := suite.doPost("http://"+suite.server.addr().String()+SupervisionURL, "supervision")
	suite.Equal("405 method not allowed\n", reply)
	suite.Equal(405, resp.StatusCode)
	suite.Equal("405 Method Not Allowed", resp.Status)
}

func (suite *HTTPServerTestSuite) doGet(url string) (*http.Response, string) {
	resp, err := http.Get(url)
	suite.Nil(err)

	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	suite.Nil(err)
	return resp, string(contents)
}

func (suite *HTTPServerTestSuite) doPost(serverURL string, msg string) (*http.Response, string) {
	resp, err := http.Post(serverURL, "data", strings.NewReader(msg))
	suite.Nil(err)

	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	suite.Nil(err)
	return resp, string(contents)
}

func replySupervision(chSupervision chan chan string, reply string) {
	chSupervisionAck := <-chSupervision
	chSupervisionAck <- reply
}

func (suite *HTTPServerTestSuite) TestHttpServerSupervision() {

	// start the "main loop" to reply to the supervision to the HTTPServer
	go replySupervision(suite.chSupervision, "I'm just fine")

	resp, reply := suite.doGet("http://" + suite.server.addr().String() + SupervisionURL)

	suite.Equal("I'm just fine", reply)
	suite.Equal(200, resp.StatusCode)
	suite.Equal("200 OK", resp.Status)
}

func (suite *HTTPServerTestSuite) TestHttpServerInvalidUrl() {
	resp, reply := suite.doPost("http://"+suite.server.addr().String()+"/invalid_url", "foo")
	suite.Equal("404 page not found\n", reply)
	suite.Equal(404, resp.StatusCode)
	suite.Equal("404 Not Found", resp.Status)
}

func readXAppNotification(chNotif chan []byte, ch chan []byte) {
	notification := <-chNotif
	ch <- notification
}

func (suite *HTTPServerTestSuite) TestHttpServerXappNotif() {
	// start the "main loop" to receive the xAppNotification message from the HTTPServer
	ch := make(chan []byte)
	go readXAppNotification(suite.chNotif, ch)

	resp, reply := suite.doPost("http://"+suite.server.addr().String()+"/vesmgr_notif/", "test data")
	suite.Equal("", reply)
	suite.Equal(200, resp.StatusCode)
	suite.Equal("200 OK", resp.Status)
	notification := <-ch
	suite.Equal([]byte("test data"), notification)
}

func (suite *HTTPServerTestSuite) TestHttpServerXappNotifInvalidOperation() {
	resp, reply := suite.doGet("http://" + suite.server.addr().String() + "/vesmgr_notif/")
	suite.Equal("405 method not allowed\n", reply)
	suite.Equal(405, resp.StatusCode)
	suite.Equal("405 Method Not Allowed", resp.Status)
}

func TestHttpServerSuite(t *testing.T) {
	suite.Run(t, new(HTTPServerTestSuite))
}
