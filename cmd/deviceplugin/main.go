/*
 * Copyright(c) 2022 Intel Corporation.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"github.com/intel/cndp_device_plugin/internal/deviceplugin"
	"github.com/intel/cndp_device_plugin/internal/logformats"
	"github.com/intel/cndp_device_plugin/internal/networking"
	logging "github.com/sirupsen/logrus"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"os"
	"os/signal"
	"syscall"
)

const (
	defaultConfigFile = "./config.json"
	devicePrefix      = "cndp"
)

type devicePlugin struct {
	pools map[string]deviceplugin.PoolManager
}

func main() {
	var configFile string

	flag.StringVar(&configFile, "config", defaultConfigFile, "Location of the device plugin configuration file")
	flag.Parse()

	logging.SetReportCaller(true)
	logging.SetFormatter(logformats.Default)

	logging.Infof("Starting CNDP Device Plugin")
	cfg, err := deviceplugin.GetConfig(configFile, networking.NewHandler())
	if err != nil {
		logging.Errorf("Error getting device plugin config: %v", err)
		logging.Errorf("Device plugin will exit")
		os.Exit(1)
	}

	dp := devicePlugin{
		pools: make(map[string]deviceplugin.PoolManager),
	}

	for _, poolConfig := range cfg.Pools {
		pm := deviceplugin.PoolManager{
			Name:          poolConfig.Name,
			Mode:          cfg.Mode,
			Devices:       make(map[string]*pluginapi.Device),
			DpAPISocket:   pluginapi.DevicePluginPath + devicePrefix + "-" + poolConfig.Name + ".sock",
			DpAPIEndpoint: devicePrefix + "-" + poolConfig.Name + ".sock",
			UpdateSignal:  make(chan bool),
			Timeout:       cfg.Timeout,
			DevicePrefix:  devicePrefix,
		}

		if err := pm.Init(poolConfig); err != nil {
			logging.Warningf("Error initializing pool: %v", pm.Name)
			logging.Errorf("%v", err)
		}

		dp.pools[poolConfig.Name] = pm
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	s := <-sigs
	logging.Infof("Received signal \"%v\"", s)
	for _, pm := range dp.pools {
		logging.Infof("Terminating %v", pm.Name)
		if err := pm.Terminate(); err != nil {
			logging.Errorf("Termination error: %v", err)
		}
	}
}