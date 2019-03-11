package main

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type collector struct {
	client *govmomi.Client
	entity *integration.Entity
	finder *find.Finder

	metricFilter string

	metricToNameMap map[int32]string
	nameToMetricMap map[string]int32
	summaryMetrics  map[string]map[string]interface{}
	hostMetricIds   []types.PerfMetricId
}

func (c *collector) initCounterMetadata() (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var perfManager mo.PerformanceManager
	err = c.client.RetrieveOne(ctx, *c.client.ServiceContent.PerfManager, nil, &perfManager)
	if err != nil {
		log.Error("Could not retrieve performance manager")
		return err
	}
	//interval := perfManager.HistoricalInterval
	//log.Debug(interval[0].SamplingPeriod)
	perfCounters := perfManager.PerfCounter

	c.metricToNameMap = make(map[int32]string)
	c.nameToMetricMap = make(map[string]int32)

	printCounters := args.LogAvailableCounters
	if printCounters {
		fmt.Printf("LogAvailableCounters FLAG ON, printing all %d available counters", len(perfCounters))
	}
	for _, perfCounter := range perfCounters {
		groupInfo := perfCounter.GroupInfo.GetElementDescription()
		nameInfo := perfCounter.NameInfo.GetElementDescription()
		fullCounterName := groupInfo.Key + "." + nameInfo.Key + "." + fmt.Sprint(perfCounter.RollupType)
		c.nameToMetricMap[fullCounterName] = perfCounter.Key
		c.metricToNameMap[perfCounter.Key] = fullCounterName
		if printCounters {
			fmt.Printf("\t %s [%d]\n", fullCounterName, perfCounter.Level)
		}
	}
	return nil
}

func (c *collector) collect(entityType string, nrEventType string, counterList []string) error {
	missingCounters := make([]string, 0)
	metricIds := make([]types.PerfMetricId, 0)
	for _, fullCounterName := range counterList {
		counterID, ok := c.nameToMetricMap[fullCounterName]
		if ok {
			metricID := types.PerfMetricId{CounterId: counterID, Instance: "*"}
			metricIds = append(metricIds, metricID)
		} else {
			missingCounters = append(missingCounters, fullCounterName)
		}
	}
	log.Warn("unable to find `%s` counters: %v", entityType, missingCounters)

	switch entityType {
	case "Host System":
		hosts, err := c.finder.HostSystemList(context.Background(), "*")
		if err != nil {
			return err
		}
		if args.Verbose {
			discoveredHosts := make([]string, 0)
			for _, host := range hosts {
				discoveredHosts = append(discoveredHosts, host.Name())
			}
			log.Info("discovered host systems: %v ", discoveredHosts)
		}
		for _, host := range hosts {
			err = c.collectMetrics(entityType, nrEventType, host.Name(), host.Reference(), metricIds)
			if err != nil {
				log.Error(err.Error())
			}
		}
	case "Virtual Machine":
		vms, err := c.finder.VirtualMachineList(context.Background(), "*")
		if err != nil {
			return err
		}
		if args.Verbose {
			discoveredVms := make([]string, 0)
			for _, vm := range vms {
				discoveredVms = append(discoveredVms, vm.Name())
			}
			log.Info("discovered virtual machines: %v ", discoveredVms)
		}
		for _, vm := range vms {
			err = c.collectMetrics(entityType, nrEventType, vm.Name(), vm.Reference(), metricIds)
			if err != nil {
				log.Error(err.Error())
			}
		}
	case "Resource Pool":
		resourcePools, err := c.finder.ResourcePoolList(context.Background(), "*")
		if err != nil {
			return err
		}
		if args.Verbose {
			discoveredResourcePools := make([]string, 0)
			for _, resourcePool := range resourcePools {
				discoveredResourcePools = append(discoveredResourcePools, resourcePool.Name())
			}
			log.Info("discovered resource pools: %v ", discoveredResourcePools)
		}
		for _, resourcePool := range resourcePools {
			err = c.collectMetrics(entityType, nrEventType, resourcePool.Name(), resourcePool.Reference(), metricIds)
			if err != nil {
				log.Error(err.Error())
			}
		}
	case "Datastore":
		datastores, err := c.finder.DatastoreList(context.Background(), "*")
		if err != nil {
			return err
		}
		if args.Verbose {
			discoveredDatastores := make([]string, 0)
			for _, datastore := range datastores {
				discoveredDatastores = append(discoveredDatastores, datastore.Name())
			}
			log.Info("discovered datastores: %v ", discoveredDatastores)
		}
		for _, datastore := range datastores {
			err = c.collectMetrics(entityType, nrEventType, datastore.Name(), datastore.Reference(), metricIds)
			if err != nil {
				log.Error(err.Error())
			}
		}
	}
	return nil
}

func (c *collector) collectMetrics(entityType, nrEventType, name string, moref types.ManagedObjectReference, metricIds []types.PerfMetricId) error {
	ctx := context.Background()
	log.Info(fmt.Sprintf("querying %s for %s", entityType, name))

	ms := c.entity.NewMetricSet(nrEventType)
	err := ms.SetMetric("name", name, metric.ATTRIBUTE)
	if err != nil {
		log.Error(err.Error())
	}
	//add in summary metrics previously collected
	summaryMetrics, ok := c.summaryMetrics[name]
	if ok {
		fmt.Println("Adding summary metrics for " + name)
		for k, v := range summaryMetrics {
			switch tv := v.(type) {
			case string:
				err = ms.SetMetric(k, tv, metric.ATTRIBUTE)
				if err != nil {
					log.Error(err.Error())
				}
			case bool:
				var val int
				if tv {
					val = 1
				}
				err = ms.SetMetric(k, val, metric.GAUGE)
				if err != nil {
					log.Error(err.Error())
				}
			case int, int64, int32, float32, float64:
				err = ms.SetMetric(k, tv, metric.GAUGE)
				if err != nil {
					log.Error(err.Error())
				}
			default:
				log.Error("unknown metric value datatype %T", v)
			}
		}
	}

	//Note about IntervalId: ESXi Servers sample performance data every 20 seconds. 20-second interval data is called instance data or real-time data
	//TODO It may be required to also specify begin and end times.
	querySpec := types.PerfQuerySpec{
		Entity:     moref,
		MaxSample:  1,
		MetricId:   metricIds,
		IntervalId: 20,
	}

	query := types.QueryPerf{
		This:      *c.client.ServiceContent.PerfManager,
		QuerySpec: []types.PerfQuerySpec{querySpec},
	}

	retrievedStats, _ := methods.QueryPerf(ctx, c.client, &query)
	if retrievedStats == nil || len(retrievedStats.Returnval) == 0 {
		log.Warn("no results returned from query execution for %s[ %s ]", entityType, name)
		return nil
	}
	singleEntityPerfStats := retrievedStats.Returnval[0]

	metricsValues := singleEntityPerfStats.(*types.PerfEntityMetric).Value
	for _, metricValue := range metricsValues {
		switch metricValueSeries := metricValue.(type) {

		case *types.PerfMetricIntSeries:
			//
			counterInfo, ok := c.metricToNameMap[metricValueSeries.Id.CounterId]
			if ok {
				if len(metricValueSeries.Value) > 1 {
					log.Warn("series contains more than one value %d \n", len(metricValueSeries.Value))
				}
				if len(metricValueSeries.Value) > 0 {
					err = ms.SetMetric(counterInfo, metricValueSeries.Value[0], metric.GAUGE)
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		default:
			log.Warn("unknown BasePerfMetricSeries type %T!\n", metricValueSeries)
		}
	}
	return nil
}
