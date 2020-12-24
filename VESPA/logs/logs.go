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
package logs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// More loggers can be added if required
// For more dynamism it can even be initialized from main.
var (
	AUDIT_LOG_HEAD = "ACTIVITY_VES_AGENT_"
	//LOGGER         = map[string]int{
	LogService = "ACTIVITY_VES_AGENT_"
	LOGGER     = map[string]int{
		"Exception": 1,
		"Error":     2,
		"Warning":   3,
		"Info":      4,
		"Debug":     5,
	}
	AUDIT_LOG_Func = map[int]string{
		1: "ConfigUpdate",
	}
)

type LogLevel int

const (
	LogLevelAll LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarning
	LogLevelError
	LogLevelException
)

type LogConfiguration struct {
	StandardOutput bool     // true to print o/p on console. by dafault false. No console print
	LogFile        bool     // true to print to file. by default false.
	DateFormat     string   //  date format. Check time package for info
	TimeFormat     string   // Time format. Check time package for info
	TimeLocation   string   // Time and Date Location. time package ex: Asia/Kolkata
	Fd             *os.File // For file operations
	PrintException bool     // If true exception type will be printed. else not
	PrintError     bool     // If true error type will be printed. else not
	PrintWarning   bool     // If true Warning type will be printed. else not
	PrintInfo      bool     // If true Info  type will be printed. else not
	PrintDebug     bool     // If true Debug type will be printed. else not
}

var LogConf LogConfiguration

// getLogLevelEnum takes log level as string and returns LogLevel enum
func getLogLevelEnum(level string) LogLevel {
	levelMap := map[string]LogLevel{
		"ALL":       LogLevelAll,
		"DEBUG":     LogLevelDebug,
		"INFO":      LogLevelInfo,
		"WARNING":   LogLevelWarning,
		"ERROR":     LogLevelError,
		"EXCEPTION": LogLevelException,
	}
	if _, ok := levelMap[level]; !ok {
		return LogLevelAll
	}
	return levelMap[level]
}

// var lJack *lumberjack.Logger

// func NewLogger(fileName string) *lumberjack.Logger {
// 	var conf types.Lmaas = *types.CimConfigObj.CimConfig.Lmaas
// 	l := &lumberjack.Logger{
// 		Filename:   fileName,
// 		MaxSize:    conf.MaxFileSizeInMB, // megabytes
// 		MaxBackups: conf.MaxBackupFiles,
// 		MaxAge:     conf.MaxAge, //days
// 		Compress:   false,       // disabled by default
// 	}
// 	return l
// }

func InitializeLogInfo(logLevel string, file_log bool) {
	LogConf.DateFormat = "YY-MM-DD"
	LogConf.TimeFormat = "SS-MM-HH-MIC"
	LogConf.TimeLocation = ""

	setLogLevel(logLevel)
	if file_log {
		LogConf.LogFile = true
		// LogConf.ConfigureLogFile()
	}
}

func setLogLevel(logLevel string) {
	if logLevel == "CONSOLE" {
		LogConf.StandardOutput = true
		logLevel = "ALL"
	}
	logLevelEnum := getLogLevelEnum(logLevel)
	if logLevelEnum <= LogLevelException {
		LogConf.PrintException = true
	}

	if logLevelEnum <= LogLevelError {
		LogConf.PrintError = true
	}

	if logLevelEnum <= LogLevelWarning {
		LogConf.PrintWarning = true
	}

	if logLevelEnum <= LogLevelInfo {
		LogConf.PrintInfo = true
	}

	if logLevelEnum <= LogLevelDebug {
		LogConf.PrintDebug = true
	}
}

func (lc *LogConfiguration) ConfigureLogFile() {
	fmt.Println("creating new file")
	GetContainerID()
	filename := os.Getenv("K8S_POD_ID") + "_" + os.Getenv("K8S_NAMESPACE") + "_vesgw-" + os.Getenv("K8S_CONTAINER_ID") + ".log"
	var err error
	_, err = os.Stat("/opt/logs")
	fmt.Println("Logs directory info : ", err)
	if os.IsNotExist(err) {
		fmt.Println("Creating logs directory under /opt/")
		os.Mkdir("/opt/logs", 0755)
	}
	_, err = os.Stat("/opt/logs/" + filename)
	fmt.Println("Log file info : ", err)
	if !(os.IsNotExist(err)) {
		fmt.Println("Log file exist : ")
		return
	}
	lc.Fd, err = os.OpenFile("/opt/logs/"+filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error while creating file : ", "/opt/logs/"+filename, err)
	}
	fmt.Println("new file created at", lc.Fd)
	// lJack = NewLogger(lc.Fd.Name())
	return
}

func Println(level, logType string, arg ...interface{}) {
	l, ok := LOGGER[level]
	if !ok {
		l = 5
	}
	lt, ok := LOGGER[logType]
	if !ok {
		return
	}
	if lt <= l {
		ts, _ := Get("", "", "")
		fmt.Print("[" + logType + "] : " + ts + " -> ")
		fmt.Print(arg)
		fmt.Print("\n")
	}
}

func (lc *LogConfiguration) PrintToFile(msg string) {
	fmt.Fprintln(lc.Fd, fmt.Sprintf("%s", msg))
	return
}

func (lc *LogConfiguration) Exception(arg ...interface{}) {
	if !lc.PrintException {
		lc.Fd.Close()
		os.Exit(1)
	}
	ts, _ := Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
	fmt.Fprintln(lc.Fd, "[EXCEPTION] : "+ts+" -> "+fmt.Sprintf("%v", arg))
	lc.Fd.Close()
	os.Exit(1)
}

func (lc *LogConfiguration) Error(arg ...interface{}) {
	if !LogConf.PrintError {
		return
	}
	ts, _ := Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
	fmt.Fprintln(lc.Fd, "[ERROR] : "+ts+" -> "+fmt.Sprintf("%v", arg))
}

func (lc *LogConfiguration) Warning(arg ...interface{}) {
	if !LogConf.PrintWarning {
		return
	}
	ts, _ := Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
	fmt.Fprintln(lc.Fd, "[WARNING] : "+ts+" -> "+fmt.Sprintf("%v", arg))
}

func (lc *LogConfiguration) Info(arg ...interface{}) {
	if !LogConf.PrintInfo {
		return
	}
	ts, _ := Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
	fmt.Fprintln(lc.Fd, "[INFO] : "+ts+" -> "+fmt.Sprintf("%v", arg))
}

func (lc *LogConfiguration) Debug(arg ...interface{}) {
	if !LogConf.PrintDebug {
		return
	}
	ts, _ := Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
	fmt.Fprintln(lc.Fd, "[DEBUG] : "+ts+" -> "+fmt.Sprintf("%v", arg))
}

// func (lc *LogConfiguration) createLogEntry(level string, arg ...interface{}) {
// 	ts, _ := utility.Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
// 	if lc.StandardOutput {
// 		fmt.Println("["+level+"] : ", ts, "->", arg)
// 	}
// 	var err error
// 	if lc.Fd != nil {
// 		_, err = os.Stat(lc.Fd.Name())
// 	}
// 	if lc.Fd == nil || os.IsNotExist(err) {
// 		fmt.Println("["+level+"] : ", ts, "->", "Recreating log file")
// 		lc.ConfigureLogFile()
// 	}
// 	if lc.LogFile {
// 		lJack.Write([]byte(fmt.Sprintf("["+level+"] : "+ts+" -> %v\n", arg...)))
// 	}
// 	return
// }

func (lc *LogConfiguration) Audit( /*activity_functionality int, */ arg ...interface{}) {
	// lc.createLogEntry(AUDIT_LOG_HEAD+AUDIT_LOG_Func[activity_functionality], arg)
	ts, _ := Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)

	fmt.Fprintln(lc.Fd, "[AUDIT] : "+ts+" -> "+fmt.Sprintf("%v", arg))
}

//GetContainerID get containerid from script
func GetContainerID() {
	out, err := exec.Command("/bin/bash", "-c", "/opt/bin/get_containerid.sh").Output()
	if err != nil {
		fmt.Println("error while getting the container id from get_containerid.sh", err)
		os.Setenv("K8S_CONTAINER_ID", "0000")
	}
	fmt.Println("out is :", string(out))
	containerID := strings.TrimSpace(string(out))
	os.Setenv("K8S_CONTAINER_ID", containerID)
}
