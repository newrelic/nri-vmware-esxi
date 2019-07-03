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
	SourceConfig         int    `default:"9" help:"Undocumented"`
	Insecure             bool   `default:"true" help:"Don't verify the server's certificate chain"`
	LogAvailableCounters bool   `default:"false" help:"[Trace] Log all available performance counters"`
}

const (
	integrationName    = "com.newrelic.vmware-esxi"
	integrationVersion = "1.2.0"

	bitHostSystemPerfMetrics     = 1 // get performance metrics for host system
	bitVirtualMachinePerfMetrics = 2 // get performance metrics for vm
	bitDatastorePerfMetrics      = 4 // get performance metrics for datastore
	bitResourcePoolPerfMetrics   = 8 // get performance metrics for resource pool

)

var (
	args argumentList

	hostCounters  []string
	vmCounters    []string
	rpoolCounters []string
	dsCounters    []string

	enableHostSystemPerfMetrics     = true
	enableVirtualMachinePerfMetrics = true
	enableDatastorePerfMetrics      = true
	enableResourcePoolPerfMetrics   = true
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

	sourceConfig := args.SourceConfig
	if (sourceConfig & bitHostSystemPerfMetrics) != 0 {
		enableHostSystemPerfMetrics = true
	} else {
		enableHostSystemPerfMetrics = false
	}
	if (sourceConfig & bitVirtualMachinePerfMetrics) != 0 {
		enableVirtualMachinePerfMetrics = true
	} else {
		enableVirtualMachinePerfMetrics = false
	}
	if (sourceConfig & bitDatastorePerfMetrics) != 0 {
		enableDatastorePerfMetrics = true
	} else {
		enableDatastorePerfMetrics = false
	}
	if (sourceConfig & bitResourcePoolPerfMetrics) != 0 {
		enableResourcePoolPerfMetrics = true
	} else {
		enableResourcePoolPerfMetrics = false
	}

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
