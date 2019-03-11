package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
	Datacenter           string `default:"default" help:"Datacenter to query for metrics. {datacenter name|default|all}. all will discover all available datacenters."`
	URL                  string `default:"https://vcenteripaddress/sdk" help:"vSphere or vCenter SDK URL"`
	Username             string `default:"" help:"The vSphere or vCenter username."`
	Password             string `default:"" help:"The vSphere or vCenter password."`
	ConfigFile           string `default:"" help:"Config file containing list of metric names(overrides default config)"`
	Insecure             bool   `default:"true" help:"Don't verify the server's certificate chain"`
	LogAvailableCounters bool   `default:"false" help:"[Trace] Log all available performance counters"`
}

const (
	integrationName    = "com.newrelic.vmware-esxi"
	integrationVersion = "1.2.0"
)

var (
	args argumentList

	hostCounters  []string
	vmCounters    []string
	rpoolCounters []string
	dsCounters    []string
)

func main() {
	var err error
	// Create Integration (also process args, so this step must be done first)
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	configFile := strings.TrimSpace(args.ConfigFile)
	datacenter := strings.TrimSpace(args.Datacenter)
	url := strings.TrimSpace(args.URL)
	username := strings.TrimSpace(args.Username)
	password := strings.TrimSpace(args.Password)
	validateSSL := true

	if configFile == "" {
		//use defaults from metrics_definition.go
		hostCounters = defaultHostCounters
		vmCounters = defaultVMCounters
		rpoolCounters = []string{}
		dsCounters = []string{}
	} else {
		err = parseConfigFile(configFile)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
	}

	// Connect and login to ESXi host or vCenter
	client, err := newClient(url, username, password, validateSSL)
	if err != nil {
		log.Error("unable to create client for " + url)
		log.Error(err.Error())
		os.Exit(3)
	}
	defer logout(client)

	err = populateMetricsAndInventory(i, client, datacenter)
	if err != nil {
		log.Error(err.Error())
		os.Exit(2)
	}

	if err := i.Publish(); err != nil {
		log.Error(err.Error())
	}
}

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
		summaryMetrics, err := collectSummaryMetrics(client, dc)
		if err != nil {
			log.Error(err.Error())
		}

		finder := find.NewFinder(client.Client, true)
		// Make future calls local to this datacenter
		finder.SetDatacenter(dc)

		collector := &collector{
			client:         client,
			entity:         entity,
			finder:         finder,
			summaryMetrics: summaryMetrics,
			metricFilter:   "*",
		}
		err = collector.initCounterMetadata()
		if err != nil {
			log.Error(err.Error())
			os.Exit(5)
		}

		err = collector.collect("Host System", "ESXHostSystemSample", hostCounters)
		if err != nil {
			log.Error("failed to collect Host System metrics: %v", err)
		}

		err = collector.collect("Virtual Machine", "ESXVirtualMachineSample", vmCounters)
		if err != nil {
			log.Error("failed to collect Virtual Machine metrics: %v", err)
		}

		err = collector.collect("Resource Pool", "ESXResourcePoolSample", rpoolCounters)
		if err != nil {
			log.Error("failed to collect Resource Pool metrics: %v", err)
		}

		err = collector.collect("Datastore", "ESXDatastoreSample", dsCounters)
		if err != nil {
			log.Error("failed to collect Datastore metrics: %v", err)
		}
	}
}
