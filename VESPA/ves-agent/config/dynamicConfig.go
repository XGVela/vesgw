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
package config

import (
	logging "VESPA/cimUtils"
	"encoding/json"
	"fmt"
	"log"
	"syscall"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/nats-io/go-nats"
)

var configStructure *VesConfigObj

// type jsonAddValue struct {
// 	jsonAddValues []*jsonAddValues
// }
// type jsonAddValues struct {
// 	op    string      `json:"op,omitempty"`
// 	path  string      `json:"path,omitempty"`
// 	value interface{} `json:"value,omitempty"`
// }

func ProcessUpdateConfig(updateConfig map[string]string, nc *nats.Conn) {
	// var jsonAdd jsonAddValue
	// value := updateConfig["config-patch"]
	// err := json.Unmarshal([]byte(value), &jsonAdd)
	// if err != nil {
	// 	fmt.Println("error ", err)
	// }
	// for _, val := range jsonAdd.jsonAddValues {
	// 	ops = append(ops, val.op)
	// 	paths = append(paths, val.path)
	// 	// if val.op == "add" {
	// 	// 	content, _ := json.Marshal(val.value)
	// 	// 	original, _ := json.Marshal(VesConfigOb)
	// 	// 	original_str :=
	// 	// 	err = json.Unmarshal(content, &configStructure)

	// 	// } else
	// 	if val.op == "replace" || val.op == "add" {
	// 		err = ApplyJsonPatch([]byte(value))
	// 	}

	// 	if err != nil {
	// 		fmt.Println("error ", err)
	// 	}
	// }

	// // sendCommitVersion(updateConfig, "1", nc)
	// // value := updateConfig["config-patch"]
	// // err := ApplyJsonPatch([]byte(value))
	// // if err != nil {
	// // 	fmt.Println("error ", err)
	// // }
	// sendCommitVersion(updateConfig, "1", nc)
	// return ops, paths
	value := updateConfig["config-patch"]
	err := ApplyJsonPatch([]byte(value))
	if err != nil {
		fmt.Println("error ", err)
	}
	sendCommitVersion(updateConfig, "1", nc)
}

func sendCommitVersion(key map[string]string, commitVersion string, nc *nats.Conn) {

	commitMsg := map[string]string{
		"change-set-key": key["change-set-key"],
		"revision":       key["revision"],
		"status":         "success",
		//"status":  os.Getenv("UPDATE_COMMIT_CONFIG_STATUS"),
		"remarks": "Successfully applied the config patch",
	}

	fmt.Println("message value", commitMsg)
	logging.LogForwarder("INFO", syscall.Gettid(), "config", "CommitMessage", "CommitMessage for conf: ", fmt.Sprintf("%v", commitMsg))
	byteData, err := json.Marshal(commitMsg)
	if err != nil {
		fmt.Println("error while marshalling the message", err)
		return
	}

	err = nc.Publish("CONFIG", byteData)
	if err != nil {
		fmt.Println("error while publishing the commit version", err)
		return
	}

	nc.Flush()
}

func ApplyJsonPatch(patch []byte) (err error) {
	log.Println("Recieved json patch", string(patch[:]))
	logging.LogForwarder("INFO", syscall.Gettid(), "config", "ApplyJsonPatch", "Applying the patch for dynamic conf request")
	decode_patch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		log.Println("Json patch decode fail", err.Error())
		return
	}
	original, _ := json.Marshal(VesConfigOb)
	fmt.Println("original after marshal", string(original))

	modified, err := decode_patch.Apply(original)
	if err != nil {
		log.Println("Json patch failed", err.Error())
		return
	}
	orig, _ := json.Marshal(modified)
	log.Println("Json patch successfull Original Value", string(orig[:]))
	logging.LogForwarder("INFO", syscall.Gettid(), "config", "ApplyJsonPatch", "Json Patch successfull Original value: "+string(original))
	// data := VesConfigObj{}
	var data VesConfigObj
	json.Unmarshal(modified, &data)
	VesConfigOb.Config.CollectorDetail = data.Config.CollectorDetail
	//testMethod()
	mod, _ := json.Marshal(data)
	VesConfigOb = &data
	log.Println("Json Patch successfull Modified value", string(mod[:]))
	logging.LogForwarder("INFO", syscall.Gettid(), "config", "ApplyJsonPatch", "Json Patch successfull Modified value: "+string(mod[:]))
	return
}
