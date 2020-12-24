/*
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
package cimUtils

import (
	fbEvent "VESPA/flatbuf/EventInterface"
	fbLog "VESPA/flatbuf/LogInterface"
	"VESPA/logs"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	fb "github.com/google/flatbuffers/go"
	"github.com/nats-io/go-nats"
)

var (
	//NatsConn nats connection string
	nc                     *nats.Conn
	NatsConn               *nats.Conn
	err                    error
	subj                   = "EVENT"
	firstFailed            = false
	firstFailedC           = make(map[string]bool)
	returnFromFailedEvent  = false
	returnFromFailedEventC = make(map[string]bool)
	msgBytes               int
	ContainerID            string
	CollectorSta           = make(map[string]string)
	CollectorStatus        = make(map[string]string)
)

//NatsConnection nats connection
func NatsConnection() {
	var urls = flag.String("s", nats.DefaultURL, "The nats server URLs (separated by comma)")

	// Connect Options.
	opts := []nats.Option{nats.Name("NATS TRL Publisher")}
	time.Sleep(30 * time.Second)
	for {
		nc, err = nats.Connect(*urls, opts...)

		if err != nil {
			log.Println("no nats connection", err)
			//log.Fatal(err)
			continue
		}
		break
	}
	NatsConn = nc
	log.Println("nats connection is successful")
	//	LogForwarder("nats connection is successful")
}

//LogForwarder forwrds the log to nats
func LogForwarder(logLevel string, pid int, funcName string, internalFunc string, args ...string) {
	ts, _ := logs.Get(logs.LogConf.DateFormat, logs.LogConf.TimeFormat, logs.LogConf.TimeLocation)
	msg := "[" + ts + "][" + logLevel + "][" + strconv.Itoa(pid) + "][" + funcName + "][" + internalFunc + "]" + " - "
	for _, val := range args {
		msg += val + " "
	}
	logs.LogConf.PrintToFile(msg)
	msgs := natsMsgConvertion(msg)
	err := nc.Publish("LOG", msgs)
	if err != nil {
		logs.LogConf.Error(err.Error())
	}
	nc.Flush()
	if logLevel == "EXCEPTION" {
		os.Exit(1)
	}
}

//natsMsgConvertion msg convertions to flatbuf
func natsMsgConvertion(logs string) []byte {
	builder := fb.NewBuilder(0)

	cntID := builder.CreateString(ContainerID)
	cntName := builder.CreateString("vesgw")
	fbPayload := builder.CreateString(logs)

	fbLog.LogMessageStart(builder)
	fbLog.LogMessageAddContainerId(builder, cntID)
	fbLog.LogMessageAddContainerName(builder, cntName)
	fbLog.LogMessageAddPayload(builder, fbPayload)
	logMsg := fbLog.LogMessageEnd(builder)

	builder.Finish(logMsg)

	return builder.FinishedBytes()
}

func ConnCheck(subj string, host string, port string) {

	// firstFailed := false
	// returnFromFailedEvent := false
	var msg = []byte{}

	// startime := time.Now()
	conn, err := net.DialTimeout("tcp", host+":"+port, 10*time.Second)
	if err != nil {
		fmt.Println("Err while connecting to prometheus host: ", host, " Port: ", port, err, time.Now())
	}
	if err != nil {
		if firstFailed == false {

			fmt.Println("Sending event for prometheus down ...........", time.Now())
			firstFailed = true
			returnFromFailedEvent = true

			msg = GetEvenFlatBufferMsg("EVENT", "PrometheusConnectivityDown", host, "prometheus")
			err = PublishLogmsg(nc, subj, msg)
			if err != nil {
				log.Fatal(err)
			}

			nc.Flush()
			//return
		}
		return

	}
	if returnFromFailedEvent == true {
		fmt.Println("Sending link up alert for Prometheus connectivity...........", time.Now())

		returnFromFailedEvent = false
		firstFailed = false
		msg = GetEvenFlatBufferMsg("EVENT", "PrometheusConnectivityUp", host, "prometheus")
		err = PublishLogmsg(nc, subj, msg)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Connected to Prometheus", conn.RemoteAddr().String())
		return

	}
	// fmt.Println("Link up ..........Connected to ", conn.RemoteAddr().String(), startime)
	// defer conn.Close()

}

func CollectorConnCheck(subj string, host string, port string, ID string) {

	// startime := time.Now()
	conn, err := net.DialTimeout("tcp", host+":"+port, 10*time.Second)
	url := host + ":" + port
	if err != nil {
		fmt.Println("Err while connecting to collector host: ", host, "Port: ", port, err, time.Now())
		if firstFailedC[ID] == false {
			CollectorStatus[ID] = "DOWN"
			// url := host + ":" + port
			ConnCheckForSimulator(subj, url, "CollectorDown")
			fmt.Println("CollectorStatus in CollectorConnCheck ", CollectorStatus)
			fmt.Println("sending err alert for collectorDown ", host, port, time.Now())
			firstFailedC[ID] = true
			returnFromFailedEventC[ID] = true
		}
		return
	}
	if returnFromFailedEventC[ID] == true {
		fmt.Println("sending collector up alert ", host, port, time.Now())
		// govel.CollectorStatus[ID] = "UP"
		// CollectorSta[ID] = "UP"
		CollectorStatus[ID] = "UP"
		ConnCheckForSimulator(subj, url, "CollectorUp")
		fmt.Println("CollectorStatus in CollectorConnCheck ", CollectorStatus)
		returnFromFailedEventC[ID] = false
		firstFailedC[ID] = false
		fmt.Printf("Connected to ", conn.RemoteAddr().String())
		return
	}
	// fmt.Println("Link up collecctor..........Connected to  \n", conn.RemoteAddr().String(), startime)
	// defer conn.Close()

}

func ConnCheckForSimulator(subj string, add string, collectorStatus string) {
	fmt.Println("Inside generating collectorDown event==========", time.Now())
	var msg = []byte{}

	startime := time.Now()

	{
		{

			fmt.Println("sending err alert for collectorStatus ..........."+collectorStatus, time.Now())
			msg = GetEvenFlatBufferMsg("EVENT", collectorStatus, add, "collector")
			err = PublishLogmsg(nc, subj, msg)
			if err != nil {
				log.Fatal(err)
			}

			nc.Flush()
			//return
		}
		return

	}

	fmt.Println("Event sent for Simulator", startime)
	// defer conn.Close()

}

func GetEvenFlatBufferMsg(subj string, eventName string, add string, eventkey string) (outmsg []byte) {
	var payload string
	//payload := "{\"log\":\"[LOG_INF][abcd234e]0-This is a sample log message1.Message is encoded in protobuf and published over NATS. The subscriber reads from NATS and writes the content into a log file. FluentBit then forward the same log to elasticsearch.\r\n\",\"stream\":\"stdout\",\"time\":" + time.Now().Format("2006-01-02 15:04:05.000") + "}"
	if subj == "LOG" {
		payload = "[" + time.Now().Format("2006-01-02 15:04:05.000") + "][LOG_INF][abcd234e]0-This is a sample log message1.Message is encoded in protobuf and published over NATS. The subscriber reads from NATS and writes the content into a log file. FluentBit then forward the same log to elasticsearch."
	} else {

		builder := fb.NewBuilder(0)

		eventName := builder.CreateString(eventName)
		containerId := builder.CreateString(ContainerID)

		moKey1 := builder.CreateString(eventkey)
		moVal1 := builder.CreateString(add)

		fbEvent.KeyValueStart(builder)
		fbEvent.KeyValueAddKey(builder, moKey1)
		fbEvent.KeyValueAddValue(builder, moVal1)
		mo1 := fbEvent.KeyValueEnd(builder)

		fbEvent.EventStartManagedObjectVector(builder, 1)
		//builder.PrependUOffsetT(mo2)
		builder.PrependUOffsetT(mo1)
		managedObject := builder.EndVector(1)

		// moKey4 := builder.CreateString("")
		// moVal4 := builder.CreateString("")

		moKey3 := builder.CreateString("host")
		moVal3 := builder.CreateString(add)

		fbEvent.KeyValueStart(builder)
		fbEvent.KeyValueAddKey(builder, moKey3)
		fbEvent.KeyValueAddValue(builder, moVal3)
		mo2 := fbEvent.KeyValueEnd(builder)

		fbEvent.EventStartAdditionalInfoVector(builder, 1)
		builder.PrependUOffsetT(mo2)
		additionalInfo := builder.EndVector(1)

		etime := time.Now().UnixNano() / int64(time.Millisecond)
		fbEvent.EventStart(builder)
		fbEvent.EventAddEventName(builder, eventName)
		fbEvent.EventAddEventTime(builder, etime)
		fbEvent.EventAddContainerId(builder, containerId)
		fbEvent.EventAddManagedObject(builder, managedObject)
		fbEvent.EventAddAdditionalInfo(builder, additionalInfo)

		eventMsg := fbEvent.EventEnd(builder)

		builder.Finish(eventMsg)

		return builder.FinishedBytes()

	}

	if msgBytes == 512 {
		payload = payload + "|" + payload
	} else if msgBytes == 1024 {
		payload = payload + "|" + payload + "|" + payload + "|" + payload
	} else if msgBytes == 2048 {
		payload = payload + "|" + payload + "|" + payload + "|" + payload + "|" + payload + "|" + payload + "|" + payload + "|" + payload
	}

	builder := fb.NewBuilder(0)

	cntID := builder.CreateString(ContainerID)
	cntName := builder.CreateString("app")
	fbPayload := builder.CreateString(payload)

	fbLog.LogMessageStart(builder)
	fbLog.LogMessageAddContainerId(builder, cntID)
	fbLog.LogMessageAddContainerName(builder, cntName)
	fbLog.LogMessageAddPayload(builder, fbPayload)
	logMsg := fbLog.LogMessageEnd(builder)

	builder.Finish(logMsg)
	return builder.FinishedBytes()
}

//PublishLogmsg log message
func PublishLogmsg(nc *nats.Conn, subj string, logmsg []byte) error {

	nc.Publish(subj, logmsg)
	//nc.Flush()
	return nc.LastError()
}
