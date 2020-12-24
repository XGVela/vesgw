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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
)

// SupervisionURL is the url where kubernetes posts alive queries
const SupervisionURL = "/supervision/"

// HTTPServer is the VesMgr HTTP server struct
type HTTPServer struct {
	listener net.Listener
}

func (s *HTTPServer) init(address string) *HTTPServer {
	var err error
	s.listener, err = net.Listen("tcp", address)
	if err != nil {
		panic("Cannot listen:" + err.Error())
	}
	return s
}

func (s *HTTPServer) start(notifPath string, notifCh chan []byte, supCh chan chan string) {
	go runHTTPServer(s.listener, notifPath, notifCh, supCh)
}

func (s *HTTPServer) addr() net.Addr {
	return s.listener.Addr()
}

func runHTTPServer(listener net.Listener, xappNotifURL string, notifCh chan []byte, supervisionCh chan chan string) {

	logger.Info("vesmgr http server serving at %s", listener.Addr())

	http.HandleFunc(xappNotifURL, func(w http.ResponseWriter, r *http.Request) {

		switch r.Method {
		case "POST":
			logger.Info("httpServer: POST in %s", xappNotifURL)
			body, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			if err != nil {
				logger.Error("httpServer: Invalid body in POST request")
				return
			}
			notifCh <- body
			return
		default:
			logger.Error("httpServer: Invalid method %s to %s", r.Method, r.URL.Path)
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	http.HandleFunc(SupervisionURL, func(w http.ResponseWriter, r *http.Request) {

		switch r.Method {
		case "GET":
			logger.Info("httpServer: GET supervision")
			supervisionAckCh := make(chan string)
			// send supervision to the main loop
			supervisionCh <- supervisionAckCh
			reply := <-supervisionAckCh
			logger.Info("httpServer: supervision ack from the main loop: %s", reply)
			fmt.Fprintf(w, reply)
			return
		default:
			logger.Error("httpServer: invalid method %s to %s", r.Method, r.URL.Path)
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
			return
		}

	})

	http.Serve(listener, nil)
}
