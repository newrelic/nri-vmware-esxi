package main

import (
	"os"
	"strings"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
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
	integrationName              = "com.newrelic.vmware-esxi"
	integrationVersion           = "1.0.7"
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
	// Create Integration (also process args, so this step must be done first)
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	// read args
	configFile := strings.TrimSpace(args.ConfigFile)
	datacenter := strings.TrimSpace(args.Datacenter)
	url := strings.TrimSpace(args.URL)
	username := strings.TrimSpace(args.Username)
	password := strings.TrimSpace(args.Password)
	validateSSL := true

	//
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
