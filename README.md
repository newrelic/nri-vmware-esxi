# DEPRECATION NOTICE
This repository has been deprecated. Use https://github.com/newrelic/nri-vsphere instead.

# New Relic Infrastructure Integration for VMWare vSphere

Reports status and metrics for vSphere server

## Disclaimer

New Relic has open-sourced this integration to enable monitoring of this technology. This integration is provided AS-IS WITHOUT WARRANTY OR SUPPORT, although you can report issues and contribute to this integration via GitHub. Support for this integration is available with an [Expert Services subscription](https://newrelic.com/expertservices).

## Requirements

vCenter SDK Endpoint enabled

## Installation

Install the vsphere monitoring plugin

```sh

cp -R bin /var/db/newrelic-infra/custom-integrations/

cp vmware-esxi-definition.yml /var/db/newrelic-infra/custom-integrations/

cp vmware-esxi-config.yml.sample  /etc/newrelic-infra/integrations.d/

```

## Configuration

In order to use the `vmware-esxi` integration it is required to configure vmware-esxi-config.yml.sample file. Firstly, rename the file to vmware-esxi-config.yml (drop the .sample extension to enable this integration).

Edit the vmware-esxi-config.yml configuration file to provide a unique instance name and valid values for (ESXi URL and login credentials) url, username and password.

Restart the infrastructure agent

```sh
sudo systemctl stop newrelic-infra

sudo systemctl start newrelic-infra
```

## Troubleshooting

Check correct functioning of the plugin by executing it from the command line

```sh
Usage of ./bin/nr-vmware-esxi:
  -datacenter string
        Datacenter to query for metrics. {datacenter name|default|all}. all will discover all available datacenters. (default "default")
  -url string
        vSphere or vCenter SDK URL (default "https://vcenteripaddress/sdk")
  -username string
        The vSphere or vCenter username.
  -password string
        The vSphere or vCenter password.
  -insecure
        Don't verify the server's certificate chain (default true)
  -log_available_counters
        [Trace] Log all available performance counters
  -metrics
        Publish metrics data.
  -pretty
        Print pretty formatted JSON.
  -source_config int
        Undocumented (default 9)
  -verbose
        Print more information to logs.
```

## Usage

You can view your data in Insights by creating your own custom NRQL queries. To
do so use **ESXHostSystemSample** and **ESXVirtualMachineSample** event types.

## Compatibility

- Supported OS: Linux
- VMware versions: Tested with v 6.7
