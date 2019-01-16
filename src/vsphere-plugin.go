package main

import (
	"context"
	"net/url"
	"strings"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"
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
	integrationName    = "com.newrelic.vsphere-plugin"
	integrationVersion = "1.0.2"
)

var args argumentList

var vmDatacenter string
var vmURL string
var vmUsername string
var vmPassword string
var validateSSL bool

var ctx context.Context

func main() {
	integration, err := sdk.NewIntegration(integrationName, integrationVersion, &args)
	fatalIfErr(err)

	vmDatacenter = strings.TrimSpace(args.Datacenter)
	vmURL = strings.TrimSpace(args.URL)
	vmUsername = strings.TrimSpace(args.Username)
	vmPassword = strings.TrimSpace(args.Password)
	validateSSL = true

	if args.All || args.Inventory {
		fatalIfErr(populateInventory(integration.Inventory))
	}

	if args.All || args.Metrics {
		fatalIfErr(populateMetrics(integration))
	}
	fatalIfErr(integration.Publish())
}

func populateMetrics(integration *sdk.Integration) error {
	// Insert here the logic of your integration to get the metrics data
	// Ex: ms.SetMetric("requestsPerSecond", 10, metric.GAUGE)
	// --
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Connect and login to ESX or vCenter
	client, err := newClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Logout(ctx)

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

func populateInventory(inventory sdk.Inventory) error {
	// Insert here the logic of your integration to get the inventory data
	// Ex: inventory.SetItem("softwareVersion", "value", "1.0.1")
	// --
	return nil
}

func setCredentials(u *url.URL, un string, pw string) {
	// Override username if provided
	if un != "" {
		var password string
		var ok bool

		if u.User != nil {
			password, ok = u.User.Password()
		}

		if ok {
			u.User = url.UserPassword(un, password)
		} else {
			u.User = url.User(un)
		}
	}

	// Override password if provided
	if pw != "" {
		var username string

		if u.User != nil {
			username = u.User.Username()
		}

		u.User = url.UserPassword(username, pw)
	}
}

// newClient creates a govmomi.Client for use in the examples
func newClient(ctx context.Context) (*govmomi.Client, error) {
	// Parse URL from string
	u, err := soap.ParseURL(vmURL)
	if err != nil {
		return nil, err
	}

	// Override username and/or password as required
	setCredentials(u, vmUsername, vmPassword)

	// Connect and log in to ESX or vCenter
	return govmomi.NewClient(ctx, u, validateSSL)
}

func populateMetricsForDatacenter(finder *find.Finder, client *govmomi.Client, dc *object.Datacenter, integration *sdk.Integration) {
	log.Debug("\n Populating metrics for datacenter [%s] \n", dc.Name())
	// Make future calls local to this datacenter
	finder.SetDatacenter(dc)
	var err error
	err = initPerfCounters(client)
	if err != nil {
		log.Fatal(err)
	}

	err = getPerfStats(client, finder, integration)
	if err != nil {
		log.Fatal(err)
	}
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
