package main

import (
	"context"
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
	// Create Integration
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	configFile := args.ConfigFile

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

	//
	if args.All() || args.Metrics {
		err := populateMetrics(i)
		if err != nil {
			log.Error(err.Error())
			os.Exit(2)
		}
	}

	if err := i.Publish(); err != nil {
		log.Error(err.Error())
	}
}

func populateMetrics(integration *integration.Integration) error {
	var cancel context.CancelFunc
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//
	vmDatacenter := strings.TrimSpace(args.Datacenter)
	vmURL := strings.TrimSpace(args.URL)
	vmUsername := strings.TrimSpace(args.Username)
	vmPassword := strings.TrimSpace(args.Password)
	validateSSL := true

	// Connect and login to ESXi host or vCenter
	client, err := newClient(ctx, vmURL, vmUsername, vmPassword, validateSSL)
	if err != nil {
		log.Error(err.Error())
		os.Exit(3)
	}
	defer logout(ctx, client)

	all := true
	finder := find.NewFinder(client.Client, all)

	var dc *object.Datacenter
	if vmDatacenter == "default" {
		// Find one and only datacenter
		dc, err = finder.DefaultDatacenter(ctx)
		if err != nil {
			return err
		}
		populateMetricsForDatacenter(ctx, finder, client, dc, integration)
	} else if vmDatacenter == "all" {
		dclist, err := finder.DatacenterList(ctx, "*")
		if err != nil {
			return err
		}
		for _, dcItem := range dclist {
			populateMetricsForDatacenter(ctx, finder, client, dcItem, integration)
		}
	} else {
		dc, err = finder.Datacenter(ctx, vmDatacenter)
		if err != nil {
			return err
		}
		populateMetricsForDatacenter(ctx, finder, client, dc, integration)
	}
	return nil
}

func populateMetricsForDatacenter(ctx context.Context, finder *find.Finder, client *govmomi.Client, dc *object.Datacenter, integration *integration.Integration) {
	log.Debug("\n Populating metrics for datacenter [%s] \n", dc.Name())

	// Create datacenter Entity
	entity, err := integration.Entity("datacenter", dc.Name())
	if err != nil {
		log.Error(err.Error())
		os.Exit(4)
	}

	// Make future calls local to this datacenter
	finder.SetDatacenter(dc)

	summaryMetrics, err := collectSummaryMetrics(client, dc)
	if err != nil {
		log.Error(err.Error())
	}

	collector := &collector{
		client:         client,
		entity:         entity,
		finder:         finder,
		summaryMetrics: summaryMetrics,
		metricFilter:   "*",
	}
	err = collector.initCounterMetadata(ctx)
	if err != nil {
		log.Error(err.Error())
		os.Exit(5)
	}

	hosts, err := getHostSystems(ctx, finder)
	if err != nil {
		log.Error(err.Error())
	}
	err = collector.collect(ctx, "Host System", "ESXHostSystemSample", hosts, hostCounters)
	if err != nil {
		log.Error("Failed to collect Host System metrics: %v\n", err)
	}

	vms, err := getVMs(ctx, finder)
	if err != nil {
		log.Error(err.Error())
	}
	err = collector.collect(ctx, "Virtual Machine", "ESXVirtualMachineSample", vms, vmCounters)
	if err != nil {
		log.Error("Failed to collect Virtual Machine metrics: %v\n", err)
	}

	resourcePools, err := getResourcePools(ctx, finder)
	if err != nil {
		log.Error(err.Error())
	}
	err = collector.collect(ctx, "Resource Pool", "ESXResourcePoolSample", resourcePools, rpoolCounters)
	if err != nil {
		log.Error("Failed to collect Resource Pool metrics: %v\n", err)
	}

	dss, err := getDatastores(ctx, finder)
	if err != nil {
		log.Error(err.Error())
	}
	err = collector.collect(ctx, "Datastore", "ESXDatastoreSample", dss, dsCounters)
	if err != nil {
		log.Error("Failed to collect Datastore metrics: %v\n", err)
	}

	/*
		a, err := finder.ManagedObjectList(ctx, "*")
		for _, v := range a {
			fmt.Println("XXX")
			fmt.Println(v.Object.Reference().Type)
			fmt.Println(v.Object.Reference().Value)
			fmt.Println(v.Path)
		}
	*/
}
