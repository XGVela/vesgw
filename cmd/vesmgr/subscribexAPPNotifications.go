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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// appmgr API
const appmgrSubsPath = "/ric/v1/subscriptions"

var errPostingFailed = errors.New("Posting subscriptions failed")
var errWrongStatusCode = errors.New("Wrong subscriptions response StatusCode")

func subscribexAppNotifications(targetURL string, subscriptions chan subscriptionNotification, timeout time.Duration, subsURL string) {
	requestBody := []byte(fmt.Sprintf(`{"maxRetries": 5, "retryTimer": 5, "eventType":"all", "targetUrl": "%v"}`, targetURL))
	req, err := http.NewRequest("POST", subsURL, bytes.NewBuffer(requestBody))
	if err != nil {
		logger.Error("Setting NewRequest failed: %s", err)
		subscriptions <- subscriptionNotification{false, err, ""}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	client.Timeout = time.Second * timeout
	var subsID string
	for {
		subsID, err = subscribexAppNotificationsClientDo(req, client)
		if err == nil {
			break
		} else if err != errPostingFailed && err != errWrongStatusCode {
			subscriptions <- subscriptionNotification{false, err, ""}
			return
		}
		time.Sleep(5 * time.Second)
	}
	subscriptions <- subscriptionNotification{true, nil, subsID}
}

func subscribexAppNotificationsClientDo(req *http.Request, client *http.Client) (string, error) {
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Posting subscriptions failed: %s", err)
		return "", errPostingFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		logger.Info("Subscriptions response StatusCode: %d", resp.StatusCode)
		logger.Info("Subscriptions response headers: %s", resp.Header)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Subscriptions response Body read failed: %s", err)
			return "", err
		}
		logger.Info("Response Body: %s", body)
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(body), &result); err != nil {
			logger.Error("json.Unmarshal failed: %s", err)
			return "", err
		}
		logger.Info("Subscription id from the response: %s", result["id"].(string))
		return result["id"].(string), nil
	}
	logger.Error("Wrong subscriptions response StatusCode: %d", resp.StatusCode)
	return "", errWrongStatusCode
}
