# New Relic Infrastructure Integration for VMWare vSphere

Reports status and metrics for vSphere server

## Requirements

VSphere REST Endpoint enabled

## Configuration

Edit the vsphere-plugin-config.yml configuration file to provide a unique instance name and valid values for (ESXi REST API) url and config_file. Also specify the -insecure flag if the ESXi host certificate is self signed, invalid or expired.

## Installation

Install the vsphere monitoring plugin

```sh

cp -R bin /var/db/newrelic-infra/custom-integrations/

cp vsphere-plugin-definition.yml /var/db/newrelic-infra/custom-integrations/

cp vsphere-plugin-config.yml.sample  /etc/newrelic-infra/integrations.d/

```

## Configuration

In order to use the `vsphere-plugin` integration it is required to configure vsphere-plugin-config.yml.sample file. Firstly, rename the file to vsphere-plugin-config.yml. Then, depending on your needs, specify all instances that you want to monitor. Once this is done, restart the Infrastructure agent.

Restart the infrastructure agent

```sh
sudo systemctl stop newrelic-infra

sudo systemctl start newrelic-infra
```

## Troubleshooting

Check correct functioning of the plugin by executing it from the command line

```sh
Usage of ./bin/nr-vsphere-plugin:
  -all
        Publish all kind of data (metrics, inventory, events).
  -events
        Publish events data.
  -insecure
        Don't verify the server's certificate chain [GOVMOMI_INSECURE]
  -inventory
        Publish inventory data.
  -metrics
        Publish metrics data.
  -pretty
        Print pretty formatted JSON.
  -config_file string
        Config file containing list of metric names(overrides default config) (default "uses inbuilt config")
  -datacenter string
        Datacenter [GOVMOMI_DATACENTER] (default "default")
  -url string
        ESX or vCenter URL [GOVMOMI_URL] (default "https://username:password@host/sdk")
  -verbose
        Print more information to logs.
```

## Usage

You can view your data in Insights by creating your own custom NRQL queries. To
do so use **ESXHostSystemSample** and **ESXVirtualMachineSample** event types.

## Compatibility

* Supported OS: Linux
* vsphere-plugin versions: Tested with v 6.7
