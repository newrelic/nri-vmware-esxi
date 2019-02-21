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
	integrationVersion = "1.1.0"
)

var (
	args argumentList
)

var vmDatacenter string
var vmURL string
var vmUsername string
var vmPassword string
var validateSSL bool
var ctx context.Context

func main() {
	// Create Integration
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	vmDatacenter = strings.TrimSpace(args.Datacenter)
	vmURL = strings.TrimSpace(args.URL)
	vmUsername = strings.TrimSpace(args.Username)
	vmPassword = strings.TrimSpace(args.Password)
	validateSSL = true

	if args.All() || args.Metrics {
		err := populateMetrics(i)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		if err := i.Publish(); err != nil {
			log.Error(err.Error())
		}
	} else if args.Events {
		err := populateEvents(i)
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
	}
}

func populateEvents(integration *integration.Integration) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Connect and login to ESXi host or vCenter
	client, err := newClient(ctx)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
	defer logout(client)

	e := NewEventListener(ctx, client, integration)
	err = e.createEventListeners()
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	return nil
}

func populateMetrics(integration *integration.Integration) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Connect and login to ESXi host or vCenter
	client, err := newClient(ctx)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
	defer logout(client)

	all := true
	finder := find.NewFinder(client.Client, all)

	var dc *object.Datacenter
	if vmDatacenter == "default" {
		// Find one and only datacenter
		dc, err = finder.DefaultDatacenter(ctx)
		if err != nil {
			return err
		}
		populateMetricsForDatacenter(finder, client, dc, integration)
	} else if vmDatacenter == "all" {
		dclist, err := finder.DatacenterList(ctx, "*")
		if err != nil {
			return err
		}
		for _, dcItem := range dclist {
			populateMetricsForDatacenter(finder, client, dcItem, integration)
		}
	} else {
		dc, err = finder.Datacenter(ctx, vmDatacenter)
		if err != nil {
			return err
		}
		populateMetricsForDatacenter(finder, client, dc, integration)
	}
	return nil
}

func populateMetricsForDatacenter(finder *find.Finder, client *govmomi.Client, dc *object.Datacenter, integration *integration.Integration) {
	log.Debug("\n Populating metrics for datacenter [%s] \n", dc.Name())

	// Create datacenter Entity
	entity, err := integration.Entity("datacenter", dc.Name())
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	// Make future calls local to this datacenter
	finder.SetDatacenter(dc)
	err = initPerfCounters(client)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	err = getPerfStats(client, finder, entity)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
