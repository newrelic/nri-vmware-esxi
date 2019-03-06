package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/newrelic/infra-integrations-sdk/log"
)

type metricDefinitions struct {
	Host                   []string
	VM                     []string
	ResourcePool           []string
	ClusterComputeResource []string
	Datastore              []string
}

func fileExists(filePath string) (exists bool) {
	exists = true

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exists = false
	}
	log.Debug("%s exists? %v", filePath, exists)
	return
}

func loadConfiguration(file string) (metricDefinitions, error) {
	var metricDefinitions metricDefinitions
	configFile, err := os.Open(file)
	defer close(configFile)
	if err != nil {
		log.Error("Error reading configuration file '%s': %v", file, err)
		return metricDefinitions, err
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&metricDefinitions)
	if err != nil {
		log.Error("Error reading configuration file '%s': %v", file, err)
		return metricDefinitions, err
	}
	return metricDefinitions, nil
}

func parseConfigFile(configFile string) error {
	log.Info(fmt.Sprintf("Reading configuration file %s", configFile))

	if !fileExists(configFile) {
		return fmt.Errorf("Error loading configuration from file. Configuration file does not exist")
	}
	metricDef, err := loadConfiguration(configFile)
	if err != nil {
		return fmt.Errorf("Error loading configuration from file. Default metric configuration will be used. (%v)", err)
	}
	hostCounters = metricDef.Host
	log.Debug("Host metrics from configuration = %v", hostCounters)
	vmCounters = metricDef.VM
	log.Debug("VM metrics from configuration= %v", vmCounters)
	rpoolCounters = metricDef.ResourcePool
	log.Debug("Resource Pool metrics from configuration= %v", rpoolCounters)

	return nil
}
