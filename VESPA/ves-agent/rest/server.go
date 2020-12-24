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

package rest

import (
	logging "VESPA/cimUtils"
	"VESPA/ves-agent/config"
	"io"
	"net/http"
	"syscall"

	"github.com/gorilla/mux"
	// log "github.com/sirupsen/logrus"
)

// Route defines a route
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.Handler
}

// StartServer is used to rest server initialization and start up
func StartServer(binAddr string, handler http.Handler) {
	config.ShutdownAlertMgrSrv()
	if handler != nil {
		logging.LogForwarder("DEBUG", syscall.Gettid(), "rest", "StartServer", "router correctly initialized for ", binAddr)
		if err := http.ListenAndServe(binAddr, handler); err != nil {
			if err != http.ErrServerClosed {
				logging.LogForwarder("EXCEPTION", syscall.Gettid(), "rest", "StartServer", "Cannot start server: ", err.Error())
			}
			logging.LogForwarder("EXCEPTION", syscall.Gettid(), "rest", "StartServer", "server is shutdown: ", err.Error())
		}
	} else {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "rest", "StartServer", "error in router initialization, handler not available")
	}
}

// NewServer configures a new router to the API
func NewServer(routes []Route) http.Handler {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}
	return router
}

// errorWrapper takes a function `f` which returns an error, and transform it
// into an `http.Handler` which replies an HTTP error if `f` returns an error
func errorWrapper(f func(resp http.ResponseWriter, req *http.Request) error) http.Handler {
	hdl := func(resp http.ResponseWriter, req *http.Request) {
		if err := f(resp, req); err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			if _, err = io.WriteString(resp, err.Error()); err != nil {
				logging.LogForwarder("ERROR", syscall.Gettid(), "rest", "errorWrapper", err.Error())

			}
		}
	}
	return http.HandlerFunc(hdl)
}
