package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

var performanceMetricIDMap map[int32]string
var performanceMetricNameMap map[string]int32

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
	var metricDef metricDefinitions
	configFile, err := os.Open(file)
	defer close(configFile)
	if err != nil {
		log.Error("Error reading configuration file '%s': %v", file, err)
		return metricDef, err
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&metricDef)
	if err != nil {
		log.Error("Error reading configuration file '%s': %v", file, err)
		return metricDef, err
	}
	return metricDef, nil
}

func initPerfCounters(client *govmomi.Client) (err error) {
	var perfManager mo.PerformanceManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &perfManager)
	if err != nil {
		log.Error("Could not retrieve performance manager")
		return err
	}
	//interval := perfManager.HistoricalInterval
	//log.Debug(interval[0].SamplingPeriod)
	perfCounters := perfManager.PerfCounter

	performanceMetricIDMap = make(map[int32]string)
	performanceMetricNameMap = make(map[string]int32)

	printCounters := args.LogAvailableCounters
	if printCounters {
		fmt.Printf("LogAvailableCounters FLAG ON, printing all %d available counters", len(perfCounters))
	}
	for _, perfCounter := range perfCounters {
		groupInfo := perfCounter.GroupInfo.GetElementDescription()
		nameInfo := perfCounter.NameInfo.GetElementDescription()
		fullCounterName := groupInfo.Key + "." + nameInfo.Key + "." + fmt.Sprint(perfCounter.RollupType)
		performanceMetricNameMap[fullCounterName] = perfCounter.Key
		performanceMetricIDMap[perfCounter.Key] = fullCounterName
		if printCounters {
			fmt.Printf("\t %s [%d]\n", fullCounterName, perfCounter.Level)
		}
	}
	return nil
}

func getPerfStats(client *govmomi.Client, finder *find.Finder, entity *integration.Entity) (err error) {
	hostDef := hostDefinition
	vmDef := vmDefinition
	resourcePoolDef := resourcePoolDefinition
	clusterComputeResourceDef := clusterComputeResourceDefinition
	datastoreDef := datastoreDefinition

	//Load custom config if configFile flagd
	configFile := args.ConfigFile
	if configFile != "" {
		if fileExists(configFile) {
			log.Info(fmt.Sprintf("Reading configuration file %s", configFile))
			metricDef, err := loadConfiguration(configFile)
			if err != nil {
				log.Error("Error loading configuration from file. Default metric configuration will be used. (%v)", err)
			} else {
				hostDef = metricDef.Host
				log.Debug("Host metrics from configuration = %v", hostDef)
				vmDef = metricDef.VM
				log.Debug("VM metrics from configuration= %v", vmDef)
				resourcePoolDef = metricDef.ResourcePool
				log.Debug("Resource Pool metrics from configuration= %v", resourcePoolDef)
				datastoreDef = metricDef.Datastore
				log.Debug("Datastore metrics from configuration= %v", datastoreDef)
			}
		} else {
			log.Fatal(fmt.Errorf("Error loading configuration from file. Configuration file does not exist"))
		}
	}

	//Query host system metrics
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		fmt.Printf("Failed to retrieve host system list: %v\n", err)
		//log err
	} else {
		hostMetricIds := make([]types.PerfMetricId, 0)
		for _, fullCounterName := range hostDef {
			counterID, ok := performanceMetricNameMap[fullCounterName]
			if ok {
				metricID := types.PerfMetricId{CounterId: counterID, Instance: hostInstancesFilter}
				hostMetricIds = append(hostMetricIds, metricID)
			} else {
				log.Warn("Unable to find Counter ID for [%s] of managed object [%s]", fullCounterName, "Host System")
			}
		}
		for _, host := range hosts {
			err = executePerfQuery(host.Name(), host.Reference(), "Host System", hostEventType, hostMetricIds, client, entity)
			if err != nil {
				log.Error("Error executing performance query: %v", err)
			}
		}
	}

	//Query virtual machine metrics
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		log.Error("Failed to retrieve virtual machine list: %v\n", err)
	} else {
		vmMetricIds := make([]types.PerfMetricId, 0)
		for _, fullCounterName := range vmDef {
			counterID, ok := performanceMetricNameMap[fullCounterName]
			if ok {
				metricID := types.PerfMetricId{CounterId: counterID, Instance: vmInstancesFilter}
				vmMetricIds = append(vmMetricIds, metricID)
			} else {
				log.Warn("Unable to find Counter ID for [%s] of managed object [%s]", fullCounterName, "Virtual Machine")
			}
		}
		for _, vm := range vms {
			err = executePerfQuery(vm.Name(), vm.Reference(), "Virtual Machine", vmEventType, vmMetricIds, client, entity)
			if err != nil {
				log.Error("Error executing performance query: %v", err)
			}
		}
	}

	//Query resource pool metrics
	resourcePools, err := finder.ResourcePoolList(ctx, "*")
	if err != nil {
		log.Error("Failed to retrieve resource pool list: %v\n", err)
	} else {
		resourcePoolMetricIds := make([]types.PerfMetricId, 0)
		for _, fullCounterName := range resourcePoolDef {
			counterID, ok := performanceMetricNameMap[fullCounterName]
			if ok {
				metricID := types.PerfMetricId{CounterId: counterID, Instance: resourcePoolInstancesFilter}
				resourcePoolMetricIds = append(resourcePoolMetricIds, metricID)
			} else {
				log.Warn("Unable to find Counter ID for [%s] of managed object [%s]", fullCounterName, "Resource Pool")
			}
		}
		for _, resourcePool := range resourcePools {
			err = executePerfQuery(resourcePool.Name(), resourcePool.Reference(), "ResourcePool", resourcePoolEventType, resourcePoolMetricIds, client, entity)
			if err != nil {
				log.Error("Error executing performance query: %v", err)
			}
		}
	}

	//Query clusterComputeResource  metrics
	clusterComputeResources, err := finder.ClusterComputeResourceList(ctx, "*")
	if err != nil {
		log.Error("Failed to retrieve Cluster Compute Resource list: %v\n", err)
	} else {
		clusterComputeResourceMetricIds := make([]types.PerfMetricId, 0)
		for _, fullCounterName := range clusterComputeResourceDef {
			counterID, ok := performanceMetricNameMap[fullCounterName]
			if ok {
				metricID := types.PerfMetricId{CounterId: counterID, Instance: clusterComputeResourceInstancesFilter}
				clusterComputeResourceMetricIds = append(clusterComputeResourceMetricIds, metricID)
			} else {
				log.Warn("Unable to find Counter ID for [%s] of managed object [%s]", fullCounterName, "Cluster Compute Resource")
			}
		}
		for _, clusterComputeResource := range clusterComputeResources {
			err = executePerfQuery(clusterComputeResource.Name(), clusterComputeResource.Reference(), "ClusterComputeResource", clusterComputeResourceEventType, clusterComputeResourceMetricIds, client, entity)
			if err != nil {
				log.Error("Error executing performance query: %v", err)
			}
		}
	}

	//Query datastore metrics
	datastores, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		log.Error("Failed to retrieve datastore list: %v\n", err)
	} else {
		datastoreMetricIds := make([]types.PerfMetricId, 0)
		for _, fullCounterName := range datastoreDef {
			counterID, ok := performanceMetricNameMap[fullCounterName]
			if ok {
				metricID := types.PerfMetricId{CounterId: counterID, Instance: datastoreInstancesFilter}
				datastoreMetricIds = append(datastoreMetricIds, metricID)
			} else {
				log.Warn("Unable to find Counter ID for [%s] of managed object [%s]", fullCounterName, "Datastore")
			}
		}
		for _, datastore := range datastores {
			err = executePerfQuery(datastore.Name(), datastore.Reference(), "Datastore", datastoreEventType, datastoreMetricIds, client, entity)
			if err != nil {
				log.Error("Error executing performance query: %v", err)
			}
		}
	}
	return nil
}

func executePerfQuery(moName string, moRef types.ManagedObjectReference, moType string, eventType string, metricIDRefs []types.PerfMetricId, client *govmomi.Client, entity *integration.Entity) (err error) {
	ms := entity.NewMetricSet(eventType)
	err = ms.SetMetric("objectName", moName, metric.ATTRIBUTE)
	if err != nil {
		log.Error(err.Error())
	}

	//Note about IntervalId: ESXi Servers sample performance data every 20 seconds. 20-second interval data is called instance data or real-time data
	//TODO It may be required to also specify begin and end times.
	querySpec := types.PerfQuerySpec{
		Entity:     moRef,
		MaxSample:  1,
		MetricId:   metricIDRefs,
		IntervalId: 20,
	}

	query := types.QueryPerf{
		This:      *client.ServiceContent.PerfManager,
		QuerySpec: []types.PerfQuerySpec{querySpec},
	}

	retrievedStats, _ := methods.QueryPerf(ctx, client, &query)
	if retrievedStats == nil || len(retrievedStats.Returnval) == 0 {
		log.Debug("No results returned from query execution for %s[ %s ]", moType, moName)
		return nil
	}
	singleEntityPerfStats := retrievedStats.Returnval[0]

	metricsValues := singleEntityPerfStats.(*types.PerfEntityMetric).Value
	for _, metricValue := range metricsValues {
		switch metricValueSeries := metricValue.(type) {

		case *types.PerfMetricIntSeries:
			//
			counterInfo, ok := performanceMetricIDMap[metricValueSeries.Id.CounterId]
			if ok {
				/*
					for _, counterValue := range v.Value {
						fmt.Printf("instance=%v, %v=%d", v.Id.Instance, counterName, counterValue)
					}
				*/
				if len(metricValueSeries.Value) > 1 {
					log.Warn("Series contains more than one value %d \n", len(metricValueSeries.Value))
				}
				if len(metricValueSeries.Value) > 0 {
					err = ms.SetMetric(counterInfo, metricValueSeries.Value[0], metric.GAUGE)
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		default:
			log.Warn("Unknown BasePerfMetricSeries type %T!\n", metricValueSeries)
		}
	}
	return nil
}
