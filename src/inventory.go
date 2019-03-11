package main

import (
	"context"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

func populateInventory(client *govmomi.Client, dc *object.Datacenter) (map[string]map[string]interface{}, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hsSummary := make(map[string]map[string]interface{})
	// Create a view of HostSystem objects
	manager := view.NewManager(client.Client)

	view, err := manager.CreateContainerView(ctx, dc.Reference(), []string{"HostSystem"}, true)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	var hss []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"summary"}, &hss)
	if err != nil {
		return nil, err
	}

	for _, hs := range hss {
		name := hs.Summary.Config.Name
		hsMetrics, ok := hsSummary[name]
		if !ok {
			hsMetrics = make(map[string]interface{})
			hsSummary[name] = hsMetrics
		}
		totalCPU := int64(hs.Summary.Hardware.CpuMhz) * int64(hs.Summary.Hardware.NumCpuCores)
		freeCPU := int64(totalCPU) - int64(hs.Summary.QuickStats.OverallCpuUsage)
		freeMemory := int64(hs.Summary.Hardware.MemorySize) - (int64(hs.Summary.QuickStats.OverallMemoryUsage) * 1024 * 1024)

		hsMetrics["hs.overallCPU"] = hs.Summary.QuickStats.OverallCpuUsage
		hsMetrics["hs.totalCPU"] = totalCPU
		hsMetrics["hs.freeCPU"] = freeCPU
		hsMetrics["freeMemory"] = freeMemory
	}
	return hsSummary, nil
}
