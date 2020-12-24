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
	"VESPA/govel"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"syscall"
	// log "github.com/sirupsen/logrus"
)

// TcaFault contains
// - the fault received by the server, to be sent to the collector
// - a channel to get the error (if any) after posting the fault.
type TcaFault struct {
	Alert    govel.Data
	Response chan error
}

// decodeJSON function used to extract Alerts from http datas
func decodeJSON(resp http.ResponseWriter, req *http.Request) govel.Data {
	dataTca := govel.Data{}
	decoder := json.NewDecoder(req.Body)

	if err := decoder.Decode(&dataTca); err != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "rest", "decodeJSON", "Bad request from  "+fmt.Sprintf("%v", req.RemoteAddr)+": "+err.Error())

		http.Error(resp, err.Error(), http.StatusBadRequest)
	}

	alertsmsg := dataTca

	return alertsmsg
}

// AlertReceiver is an handler to manage  http POST alert
func AlertReceiver(alertCh chan TcaFault) http.Handler {
	hd1 := func(resp http.ResponseWriter, req *http.Request) error {
		contentType := req.Header.Get("Content-Type")
		if contentType != "application/json" {
			logging.LogForwarder("ERROR", syscall.Gettid(), "rest", "decodeJSON", "content-type"+contentType+"not managed")

			//resp.WriteHeader(http.StatusInternalServerError)
			return errors.New("content-type %s not managed")
		}
		alertsmsg := decodeJSON(resp, req)
		errorCh := make(chan error)
		message := TcaFault{Alert: alertsmsg, Response: errorCh}
		// Non blocking write, to avoid a dead lock situation

		select {
		case alertCh <- message:
			//wait for PostEvent result
			err := <-errorCh
			if err != nil {
				logging.LogForwarder("ERROR", syscall.Gettid(), "rest", "decodeJSON", "Cannot process alert: "+err.Error())
				return err
			}
		default:
			err := errors.New(fmt.Sprintf("Alert could not be sent to a channel %v", alertsmsg.MavData.Domain))
			logging.LogForwarder("WARNING", syscall.Gettid(), "rest", "decodeJSON", err.Error())
			return err
		}
		return nil
	}
	return errorWrapper(hd1)
}
