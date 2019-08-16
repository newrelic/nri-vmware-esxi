package main

import (
	"context"
	"fmt"
	"os"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

func populateMetricsAndInventory(i *integration.Integration, client *govmomi.Client, datacenter string) error {
	all := true
	finder := find.NewFinder(client.Client, all)

	if datacenter == "default" {
		// Find one and only datacenter
		dc, err := finder.DefaultDatacenter(context.Background())
		if err != nil {
			return err
		}
		populateMetricsAndInventoryForDC(i, client, dc)
	} else if datacenter == "all" {
		dclist, err := finder.DatacenterList(context.Background(), "*")
		if err != nil {
			return err
		}
		for _, dc := range dclist {
			populateMetricsAndInventoryForDC(i, client, dc)
		}
	} else {
		dc, err := finder.Datacenter(context.Background(), datacenter)
		if err != nil {
			return err
		}
		populateMetricsAndInventoryForDC(i, client, dc)
	}
	return nil
}

func populateMetricsAndInventoryForDC(integration *integration.Integration, client *govmomi.Client, dc *object.Datacenter) {
	// Create datacenter Entity
	entity, err := integration.Entity("datacenter", dc.Name())
	if err != nil {
		log.Error(err.Error())
		os.Exit(4)
	}

	/*
		if args.All() || args.Events {
			log.Info("populating inventory for datacenter [%s]", dc.Name())
			//TODO
			setupEvents(client, dc)

		}
	*/

	if args.All() || args.Inventory {
		log.Info("populating inventory for datacenter [%s]", dc.Name())
		//TODO
		hostMetrics, err := populateInventory(client, dc)
		if err != nil {
			log.Error(err.Error())
		}
		fmt.Println(hostMetrics)
	}

	if args.All() || args.Metrics {
		log.Info("populating metrics for datacenter [%s]", dc.Name())

		//init summary collector
		summaryCollector := &summaryCollector{
			client: client,
			entity: entity,
			dc:     dc,
		}

		//init performance collector
		finder := find.NewFinder(client.Client, true)
		// Make future calls local to this datacenter
		finder.SetDatacenter(dc)

		summaryMetrics, err := collectDatastoreSummaryAttributes(client, dc)
		if err != nil {
			log.Error(err.Error())
		}
		perfCollector := &perfCollector{
			client:         client,
			entity:         entity,
			finder:         finder,
			summaryMetrics: summaryMetrics,
			metricFilter:   "*",
		}

		err = perfCollector.initCounterMetadata()
		if err != nil {
			log.Error(err.Error())
			os.Exit(5)
		}

		if enableHostSystemPerfMetrics {
			err = perfCollector.collect("Host System", "ESXHostSystemSample", hostCounters)
			if err != nil {
				log.Error("failed to collect Host System metrics: %v", err)
			}
		} else {
			err = summaryCollector.collectHostMetrics("ESXHostSystemSample")
			if err != nil {
				log.Error("failed to collect Host System metrics: %v", err)
			}
		}

		if enableVirtualMachinePerfMetrics {
			err = perfCollector.collect("Virtual Machine", "ESXVirtualMachineSample", vmCounters)
			if err != nil {
				log.Error("failed to collect Virtual Machine metrics: %v", err)
			}
		} else {
			err = summaryCollector.collectVMMetrics("ESXVirtualMachineSample")
			if err != nil {
				log.Error("failed to collect Virtual Machine metrics: %v", err)
			}
		}

		if enableResourcePoolPerfMetrics {
			err = perfCollector.collect("Resource Pool", "ESXResourcePoolSample", rpoolCounters)
			if err != nil {
				log.Error("failed to collect Resource Pool metrics: %v", err)
			}
		} else {
			err = summaryCollector.collectResourcePoolMetrics("ESXResourcePoolSample")
			if err != nil {
				log.Error("failed to collect Resource Pool metrics: %v", err)
			}
		}

		if enableDatastorePerfMetrics {
			err = perfCollector.collect("Datastore", "ESXDatastoreSample", dsCounters)
			if err != nil {
				log.Error("failed to collect Datastore metrics: %v", err)
			}
		} else {
			err = summaryCollector.collectDSMetrics("ESXDatastoreSample")
			if err != nil {
				log.Error("failed to collect Datastore metrics: %v", err)
			}
		}
	}
}
