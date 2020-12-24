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
	"VESPA/govel"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"

	// log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// List of required configuration parameters
var (
	requiredConfs = []string{"PrimaryCollector.FQDN", "PrimaryCollector.User", "PrimaryCollector.Password"}
	configName    = "ves-agent"
	configPaths   = []string{"/etc/ves-agent/", "."}
	//configPaths = []string{"C:/etc/ves-agent/", "."}
	AlertMgrSrv http.Server
)

func ShutdownAlertMgrSrv() {
	if err := AlertMgrSrv.Shutdown(context.Background()); err != nil {
		fmt.Println("Error while shutting down server", err)
	} else {
		fmt.Println("Shutting Down Server")
	}
}

func setFlags(flagSet *pflag.FlagSet) {
	flagSet.String("PrimaryCollector.ServerRoot", "", "path before the /eventListener part of the POST URL")
	flagSet.StringP("PrimaryCollector.FQDN", "f", "localhost", "VES Collector FQDN")
	flagSet.IntP("PrimaryCollector.Port", "p", 8443, "VES Collector Port")
	flagSet.BoolP("PrimaryCollector.Secure", "S", false, "Use HTTPS for VES collector")
	flagSet.StringP("PrimaryCollector.Topic", "t", "", "VES Collector Topic")
	flagSet.StringP("PrimaryCollector.User", "u", "", "VES Username")
	flagSet.StringP("PrimaryCollector.Password", "k", "", "VES Password")
	flagSet.String("PrimaryCollector.PassPhrase", "", "VES PassPhrase")
	flagSet.String("BackupCollector.ServerRoot", "", "path before the /eventListener part of the POST URL")
	flagSet.String("BackupCollector.FQDN", "", "VES Collector FQDN")
	flagSet.Int("BackupCollector.Port", 0, "VES Collector Port")
	flagSet.Bool("BackupCollector.Secure", false, "Use HTTPS for VES collector")
	flagSet.String("BackupCollector.Topic", "", "VES Collector Topic")
	flagSet.String("BackupCollector.User", "", "VES Username")
	flagSet.String("BackupCollector.Password", "", "VES Password")
	flagSet.String("BackupCollector.PassPhrase", "", "VES PassPhrase")
	flagSet.DurationP("Heartbeat.DefaultInterval", "i", 60*time.Second, "VES heartbeat interval")
	flagSet.StringP("Measurement.DomainAbbreviation", "d", "Measurement", "Domain Abbreviation")
	flagSet.DurationP("Measurement.DefaultInterval", "m", 60*time.Second, "Measurement interval")
	flagSet.String("Measurement.Prometheus.Address", "http://0.0.0.0:9090", "Base url to of Prometheus server's API")
	flagSet.Duration("Measurement.MaxBufferingDuration", time.Hour, "Maximum timeframe size of buffering")
	flagSet.IntP("Event.MaxSize", "s", 200, "Max Event Size")
	retrieveReportingEntityName(flagSet)
	flagSet.DurationP("Event.RetryInterval", "r", 10*time.Second, "VES heartbeat retry interval")
	flagSet.IntP("Event.MaxMissed", "a", 3, "Missed heartbeats until switching collector")
	flagSet.String("AlertManager.Bind", "0.0.0.0:9081", "Alert Manager Bind address")
	flagSet.String("AlertManager.Path", "/alerts", "Alert Manager Path")
	flagSet.String("AlertManager.User", "", "Alert Manager Username")
	flagSet.String("AlertManager.Password", "", "Alert Manager Password")
	flagSet.String("Cluster.ID", "", "Override the cluster's node ID")
	flagSet.StringP("DataDir", "D", "/var/lib/ves-agent/data", "Path to directory where to store data")
	flagSet.Bool("Debug", false, "Activate debug traces")
}

// InitConf initilize the config store from config file, env and cli variables.
func InitConf(conf *VESAgentConfiguration) error {

	//bind env variable
	viper.SetEnvPrefix("ves")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	// Viper will check for an environment variable any time a viper.Get request is made
	viper.AutomaticEnv()

	//read default config file
	viper.SetConfigName(configName)
	for _, v := range configPaths {
		viper.AddConfigPath(v)
	}

	if err := viper.ReadInConfig(); err != nil {
		if reflect.TypeOf(err).String() == "viper.ConfigFileNotFoundError" {
			logging.LogForwarder("WARNING", syscall.Gettid(), "config", "InitConf", "Config file not found. Go on...")

		} else {
			return err
		}
	}

	//bind arguments variable
	flagSet := pflag.NewFlagSet("conf", pflag.ExitOnError)
	setFlags(flagSet)
	if err := flagSet.Parse(os.Args); err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "config", "InitConf", err.Error())
	}
	if err := viper.BindPFlags(flagSet); err != nil {
		logging.LogForwarder("EXCEPTION", syscall.Gettid(), "config", "InitConf", err.Error())
	}

	//check required values
	// for _, v := range requiredConfs {
	// 	if !viper.IsSet(v) || viper.GetString(v) == "" {
	// 		return errors.New("Missing required configuration parameter: " + v)
	// 	}
	// }
	// if viper.IsSet("BackupCollector.FQDN") && viper.GetString("BackupCollector.FQDN") != "" {
	// 	if !viper.IsSet("BackupCollector.User") || viper.GetString("BackupCollector.User") == "" {
	// 		return errors.New("Missing User for BackupCollector")
	// 	}
	// 	if !viper.IsSet("BackupCollector.Password") || viper.GetString("BackupCollector.Password") == "" {
	// 		return errors.New("Missing Password for BackupCollector")
	// 	}
	// }

	// Viper will check in the following order: override, flag, env, config file, key/value store, default
	return viper.Unmarshal(conf)
}

func retrieveReportingEntityName(flagSet *pflag.FlagSet) {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = exec.Command("hostname").Output()
	} else {
		out, err = exec.Command("hostname", "-s").Output()
	}
	if err != nil {
		logging.LogForwarder("WARNING", syscall.Gettid(), "config", "InitConf", "Cannot retrieve hostname: %s", err.Error())

	} else {
		hostname := strings.TrimSuffix(string(out), "\n")
		flagSet.StringP("Event.ReportingEntityName", "H", hostname, "Reporting entity name to add to event's header")
	}
}

type NFMetricConfigRules struct {
	Measurement MeasurementConfiguration `mapstructure:"measurement,omitempty"`
	Event       govel.EventConfiguration `mapstructure:"event,omitempty"`
}

type NFMetricConfigRulesWithoutNfc struct {
	Measurement MeasurementConfiguration           `mapstructure:"measurement,omitempty"`
	Event       govel.EventConfigurationWithoutNfc `mapstructure:"event,omitempty"`
}

//ReadNFMetricFile read nf metric files
func ReadNFMetricFile(namespace string, ruleConfig *NFMetricConfigRulesWithoutNfc) error {
	logging.LogForwarder("INFO", syscall.Gettid(), "config", "ReadNFMetricFile", "filename is :", namespace)
	/*
		replacer := strings.NewReplacer(".", "_")
		viper.SetEnvKeyReplacer(replacer)
		viper.AddConfigPath("./nfs/")

		//read  config file
		viper.SetConfigName(namespace)
		if err := viper.ReadInConfig(); err != nil {
			if reflect.TypeOf(err).String() == "viper.ConfigFileNotFoundError" {
				logging.LogForwarder("INFO", syscall.Gettid(), "config", "ReadNFMetricFile", "Config file not found. Go on...")

				return errors.New("fileNotFound")
			} else {
				logging.LogForwarder("ERROR", syscall.Gettid(), "config", "ReadNFMetricFile", "error while reading the file", err.Error())

				return errors.New("erroeReadingfile")
			}
		}

		//unmarshal it to struct
		unmarErr := viper.Unmarshal(&ruleConfig)
		if unmarErr != nil {
			logging.LogForwarder("ERROR", syscall.Gettid(), "config", "ReadNFMetricFile", "error while unmarshalling", unmarErr.Error())

		}
	*/
	fileName := "./nfs/" + namespace + ".yaml"
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			logging.LogForwarder("INFO", syscall.Gettid(), "config", "ReadNFMetricFile", "Config file not found. Go on...")
			return errors.New("fileNotFound")
		} else {
			logging.LogForwarder("ERROR", syscall.Gettid(), "config", "ReadNFMetricFile", "error while reading the file", err.Error())
			return errors.New("errorReadingfile")
		}
	}
	unmarErr := yaml.Unmarshal(data, ruleConfig)
	if unmarErr != nil {
		logging.LogForwarder("ERROR", syscall.Gettid(), "config", "ReadNFMetricFile", "error while unmarshalling", unmarErr.Error())
		return errors.New("errorunmarshallingfile")
	}
	logging.LogForwarder("INFO", syscall.Gettid(), "config", "ReadNFMetricFile", fmt.Sprintf("rules : %v", ruleConfig.Measurement))

	return nil
}
