package main

import (
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

type managedEntity interface {
	Name() string
	Reference() types.ManagedObjectReference
}

type collector struct {
	client       *govmomi.Client
	entity       *integration.Entity
	finder       *find.Finder
	metricFilter string
}

var performanceMetricIDMap map[int32]string
var performanceMetricNameMap map[string]int32

func (c *collector) initCounterMetadata() (err error) {
	var perfManager mo.PerformanceManager
	err = c.client.RetrieveOne(ctx, *c.client.ServiceContent.PerfManager, nil, &perfManager)
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

func (c *collector) collect(entityType string, nrEventType string, instances []managedEntity, counterList []string) error {
	var err error
	perfMetricIds := make([]types.PerfMetricId, 0)
	for _, fullCounterName := range counterList {
		counterID, ok := performanceMetricNameMap[fullCounterName]
		if ok {
			metricID := types.PerfMetricId{CounterId: counterID, Instance: c.metricFilter}
			perfMetricIds = append(perfMetricIds, metricID)
		} else {
			log.Warn("Unable to find [%s] counter: [%s]", entityType, fullCounterName)
		}
	}
	for _, instance := range instances {
		log.Debug(fmt.Sprintf("Querying %s for %s", entityType, instance.Name()))

		ms := c.entity.NewMetricSet(nrEventType)
		err = ms.SetMetric("objectName", instance.Name(), metric.ATTRIBUTE)
		if err != nil {
			log.Error(err.Error())
		}

		//Note about IntervalId: ESXi Servers sample performance data every 20 seconds. 20-second interval data is called instance data or real-time data
		//TODO It may be required to also specify begin and end times.
		querySpec := types.PerfQuerySpec{
			Entity:     instance.Reference(),
			MaxSample:  1,
			MetricId:   perfMetricIds,
			IntervalId: 20,
		}

		query := types.QueryPerf{
			This:      *c.client.ServiceContent.PerfManager,
			QuerySpec: []types.PerfQuerySpec{querySpec},
		}

		retrievedStats, _ := methods.QueryPerf(ctx, c.client, &query)
		if retrievedStats == nil || len(retrievedStats.Returnval) == 0 {
			log.Debug("No results returned from query execution for %s[ %s ]", entityType, instance.Name())
			fmt.Println("nothing returned")
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
	}
	return nil
}
