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
package zk

import (
	"fmt"
	"os"
	"time"

	"github.com/curator-go/curator"
)

var (
	ZKClient curator.CuratorFramework
)

func CreateClient() {
	fmt.Println("Establish zookeeper client connection.. ")
	zkEndPoint := os.Getenv("ZK_SVC_FQDN")

	ZKClient = GetClient(zkEndPoint)
	ZKClient.Start()
	fmt.Println("ZK curator created..", ZKClient)
}

func GetClient(connString string) curator.CuratorFramework {

	retryPolicy := curator.NewExponentialBackoffRetry(time.Second, 3, 15*time.Second)
	return curator.NewClient(connString, retryPolicy)
}

func Create(client curator.CuratorFramework, path string, payload []byte) (string, error) {
	return client.Create().CreatingParentsIfNeeded().ForPathWithData(path, payload)
}

func Update(client curator.CuratorFramework, path string, payload []byte) {
	// this will create the given ZNode with the given data
	fmt.Println("Inside zk client create path....", path)
	fmt.Println("Inside zk client create payload....", payload)
	//return client.Create().ForPathWithData(path, payload)
	if PathExist(client, path) {
		fmt.Println("Path already exist, setting data...")
		client.SetData().ForPathWithData(path, payload)
	} else {
		fmt.Println("Path not exist, Create path and set data...")
		str, err := Create(client, path, payload)
		if err != nil {
			fmt.Println("Error from ZK create node...", err.Error())
		} else {
			fmt.Println("Node Create successfull ...", str)
		}
	}

}

func SetData(client curator.CuratorFramework, path string, payload []byte) (string, error) {
	// this will create the given ZNode with the given data
	fmt.Println("Inside zk client create path....", path)
	fmt.Println("Inside zk client create payload....", payload)
	return client.Create().ForPathWithData(path, payload)
	//return client.Create().CreatingParentsIfNeeded().ForPath(path)
}

func Delete(client curator.CuratorFramework, path string, version int32) error {
	return client.Delete().DeletingChildrenIfNeeded().WithVersion(version).ForPath(path)
}

func GetData(client curator.CuratorFramework, path string) []byte {
	data, err := client.GetData().ForPath(path)
	if err != nil {
		fmt.Println("Error while getting data...", err.Error())
	} else {
		fmt.Println("Data returned from getdata...", data)
	}
	return data
}
func PathExist(client curator.CuratorFramework, path string) bool {
	stat, err := client.CheckExists().ForPath(path)
	if err != nil {
		fmt.Println("Error while fetching path stat..", err.Error())
		return false
	} else if stat == nil {
		return false
	} else {
		fmt.Println("Returned path stat..", stat)
		return true
	}
}
