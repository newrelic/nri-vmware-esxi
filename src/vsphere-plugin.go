package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/sdk"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
	ConfigFile           string `default:"nil" help:"Config file containing list of metric names(overrides default config)"`
	LogAvailableCounters bool   `default:"false" help:"[Trace] Log all available performance counters"`
}

const (
	integrationName    = "com.newrelic.vsphere-plugin"
	integrationVersion = "0.1.0"
)

var args argumentList

const (
	envDataCenter = "GOVMOMI_DATACENTER"
	envURL        = "GOVMOMI_URL"
	envUserName   = "GOVMOMI_USERNAME"
	envPassword   = "GOVMOMI_PASSWORD"
	envInsecure   = "GOVMOMI_INSECURE"
)

var dataCenterDescription = fmt.Sprintf("Datacenter [%s]", envDataCenter)
var dataCenterFlag = flag.String("datacenter", getEnvString(envDataCenter, "default"), dataCenterDescription)

var urlDescription = fmt.Sprintf("ESX or vCenter URL [%s]", envURL)
var urlFlag = flag.String("url", getEnvString(envURL, "https://username:password@host"+vim25.Path), urlDescription)

var insecureDescription = fmt.Sprintf("Don't verify the server's certificate chain [%s]", envInsecure)
var insecureFlag = flag.Bool("insecure", getEnvBool(envInsecure, false), insecureDescription)

var ctx context.Context

// getEnvString returns string from environment variable.
func getEnvString(v string, def string) string {
	r := os.Getenv(v)
	if r == "" {
		return def
	}
	return r
}

// getEnvBool returns boolean from environment variable.
func getEnvBool(v string, def bool) bool {
	r := os.Getenv(v)
	if r == "" {
		return def
	}

	switch strings.ToLower(r[0:1]) {
	case "t", "y", "1":
		return true
	}

	return false
}

func processOverride(u *url.URL) {
	envUsername := os.Getenv(envUserName)
	envPassword := os.Getenv(envPassword)

	// Override username if provided
	if envUsername != "" {
		var password string
		var ok bool

		if u.User != nil {
			password, ok = u.User.Password()
		}

		if ok {
			u.User = url.UserPassword(envUsername, password)
		} else {
			u.User = url.User(envUsername)
		}
	}

	// Override password if provided
	if envPassword != "" {
		var username string

		if u.User != nil {
			username = u.User.Username()
		}

		u.User = url.UserPassword(username, envPassword)
	}
}

// newClient creates a govmomi.Client for use in the examples
func newClient(ctx context.Context) (*govmomi.Client, error) {
	flag.Parse()

	// Parse URL from string
	u, err := soap.ParseURL(*urlFlag)
	if err != nil {
		return nil, err
	}

	// Override username and/or password as required
	processOverride(u)

	// Connect and log in to ESX or vCenter
	return govmomi.NewClient(ctx, u, *insecureFlag)
}

func populateInventory(inventory sdk.Inventory) error {
	// Insert here the logic of your integration to get the inventory data
	// Ex: inventory.SetItem("softwareVersion", "value", "1.0.1")
	// --
	return nil
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
	if *dataCenterFlag == "default" {
		// Find one and only datacenter
		dc, err = finder.DefaultDatacenter(ctx)
		if err != nil {
			return err
		}
		populateMetricsForDatacenter(finder, client, dc, integration)
	} else if *dataCenterFlag == "all" {
		dclist, err := finder.DatacenterList(ctx, "*")
		if err != nil {
			return err
		}
		for _, dcItem := range dclist {
			populateMetricsForDatacenter(finder, client, dcItem, integration)
		}
	} else {
		dc, err = finder.Datacenter(ctx, *dataCenterFlag)
		if err != nil {
			return err
		}
		populateMetricsForDatacenter(finder, client, dc, integration)
	}
	return nil
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

func main() {
	integration, err := sdk.NewIntegration(integrationName, integrationVersion, &args)
	fatalIfErr(err)

	if args.All || args.Inventory {
		fatalIfErr(populateInventory(integration.Inventory))
	}

	if args.All || args.Metrics {
		fatalIfErr(populateMetrics(integration))
	}
	fatalIfErr(integration.Publish())
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
