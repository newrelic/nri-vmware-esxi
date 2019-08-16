package main

import (
	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/units"

	"github.com/vmware/govmomi/view"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type summaryCollector struct {
	client *govmomi.Client
	entity *integration.Entity
	dc     *object.Datacenter
}

func (c *summaryCollector) collectHostMetrics(nrEventType string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create a view of host objects
	manager := view.NewManager(c.client.Client)

	view, err := manager.CreateContainerView(ctx, c.dc.Reference(), []string{"HostSystem"}, true)
	if err != nil {
		return err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	var hss []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"summary"}, &hss)
	if err != nil {
		return err
	}

	for _, hs := range hss {
		hsName := hs.Summary.Config.Name
		ms := c.entity.NewMetricSet(nrEventType)
		err := ms.SetMetric("name", hsName, metric.ATTRIBUTE)
		if err != nil {
			log.Error(err.Error())
		}

		totalCPU := int64(hs.Summary.Hardware.CpuMhz) * int64(hs.Summary.Hardware.NumCpuCores)
		freeCPU := int64(totalCPU) - int64(hs.Summary.QuickStats.OverallCpuUsage)
		freeMemory := int64(hs.Summary.Hardware.MemorySize) - (int64(hs.Summary.QuickStats.OverallMemoryUsage) * 1024 * 1024)
		memoryUsage := (units.ByteSize(hs.Summary.QuickStats.OverallMemoryUsage)) * 1024 * 1024
		memorySize := units.ByteSize(hs.Summary.Hardware.MemorySize)
		overallCPUUsage := hs.Summary.QuickStats.OverallCpuUsage

		_ = ms.SetMetric("totalCPU", totalCPU, metric.GAUGE)
		_ = ms.SetMetric("freeCPU", freeCPU, metric.GAUGE)
		_ = ms.SetMetric("memoryUsage", memoryUsage, metric.GAUGE)
		_ = ms.SetMetric("memorySize", memorySize, metric.GAUGE)
		_ = ms.SetMetric("freeMemory", freeMemory, metric.GAUGE)
		_ = ms.SetMetric("overallCPUUsage", overallCPUUsage, metric.GAUGE)
	}

	return nil
}

func (c *summaryCollector) collectDSMetrics(nrEventType string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create a view of Datastore objects
	manager := view.NewManager(c.client.Client)

	view, err := manager.CreateContainerView(ctx, c.dc.Reference(), []string{"Datastore"}, true)
	if err != nil {
		return err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	var dss []mo.Datastore
	err = view.Retrieve(ctx, []string{"Datastore"}, []string{"summary"}, &dss)
	if err != nil {
		return err
	}

	for _, ds := range dss {
		dsName := ds.Summary.Name
		ms := c.entity.NewMetricSet(nrEventType)
		err := ms.SetMetric("name", dsName, metric.ATTRIBUTE)
		if err != nil {
			log.Error(err.Error())
		}

		_ = ms.SetMetric("ds.type", ds.Summary.Type, metric.ATTRIBUTE)
		_ = ms.SetMetric("ds.url", ds.Summary.Url, metric.ATTRIBUTE)
		_ = ms.SetMetric("ds.capacity", float64(ds.Summary.Capacity)/(1<<30), metric.GAUGE)
		_ = ms.SetMetric("ds.freespace", float64(ds.Summary.FreeSpace)/(1<<30), metric.GAUGE)
		_ = ms.SetMetric("ds.uncommitted", float64(ds.Summary.Uncommitted)/(1<<30), metric.GAUGE)
		_ = ms.SetMetric("ds.accessible", ds.Summary.Accessible, metric.ATTRIBUTE)

		switch info := ds.Info.(type) {
		case *types.NasDatastoreInfo:
			_ = ms.SetMetric("ds.nas.remoteHost", info.Nas.RemoteHost, metric.ATTRIBUTE)
			_ = ms.SetMetric("ds.nas.remotePath", info.Nas.RemotePath, metric.ATTRIBUTE)
		}
	}
	return nil
}

func (c *summaryCollector) collectVMMetrics(nrEventType string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create a view of VirtualMachine objects
	manager := view.NewManager(c.client.Client)

	view, err := manager.CreateContainerView(ctx, c.dc.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		return err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	var vms []mo.VirtualMachine
	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary"}, &vms)
	if err != nil {
		return err
	}

	for _, vm := range vms {
		vmConfig := vm.Summary.Config
		ms := c.entity.NewMetricSet(nrEventType)
		_ = ms.SetMetric("name", vmConfig.Name, metric.ATTRIBUTE)

		_ = ms.SetMetric("guestFullName", vmConfig.GuestFullName, metric.GAUGE)
		_ = ms.SetMetric("memorySize", vmConfig.MemorySizeMB, metric.GAUGE)
		_ = ms.SetMetric("memorySize", vmConfig.MemorySizeMB, metric.GAUGE)

		vmQuickStats := vm.Summary.QuickStats
		_ = ms.SetMetric("balloonedMemory", vmQuickStats.BalloonedMemory, metric.GAUGE)
		_ = ms.SetMetric("compressedMemory", vmQuickStats.CompressedMemory, metric.GAUGE)
		_ = ms.SetMetric("consumedOverheadMemory", vmQuickStats.ConsumedOverheadMemory, metric.GAUGE)
		_ = ms.SetMetric("distributedCpuEntitlement", vmQuickStats.DistributedCpuEntitlement, metric.GAUGE)
		_ = ms.SetMetric("distributedMemoryEntitlement", vmQuickStats.DistributedMemoryEntitlement, metric.GAUGE)
		_ = ms.SetMetric("guestMemoryUsage", vmQuickStats.GuestMemoryUsage, metric.GAUGE)
		_ = ms.SetMetric("hostMemoryUsage", vmQuickStats.HostMemoryUsage, metric.GAUGE)
		_ = ms.SetMetric("overallCpuDemand", vmQuickStats.OverallCpuDemand, metric.GAUGE)
		_ = ms.SetMetric("overallCpuUsage", vmQuickStats.OverallCpuUsage, metric.GAUGE)
		_ = ms.SetMetric("privateMemory", vmQuickStats.PrivateMemory, metric.GAUGE)
		_ = ms.SetMetric("sharedMemory", vmQuickStats.SharedMemory, metric.GAUGE)
		_ = ms.SetMetric("ssdSwappedMemory", vmQuickStats.SsdSwappedMemory, metric.GAUGE)
		_ = ms.SetMetric("staticCpuEntitlement", vmQuickStats.StaticCpuEntitlement, metric.GAUGE)
		_ = ms.SetMetric("staticMemoryEntitlement", vmQuickStats.StaticMemoryEntitlement, metric.GAUGE)
		_ = ms.SetMetric("swappedMemory", vmQuickStats.SwappedMemory, metric.GAUGE)

		switch vm.Summary.Runtime.PowerState {
		case types.VirtualMachinePowerStatePoweredOff:
			_ = ms.SetMetric("powerState", 0, metric.GAUGE)
		case types.VirtualMachinePowerStatePoweredOn:
			_ = ms.SetMetric("powerState", 2, metric.GAUGE)
		case types.VirtualMachinePowerStateSuspended:
			_ = ms.SetMetric("powerState", 1, metric.GAUGE)
		}
	}
	return nil
}

func (c *summaryCollector) collectResourcePoolMetrics(nrEventType string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create a view of ResourcePool objects
	manager := view.NewManager(c.client.Client)

	view, err := manager.CreateContainerView(ctx, c.dc.Reference(), []string{"ResourcePool"}, true)
	if err != nil {
		return err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	var rps []mo.ResourcePool
	err = view.Retrieve(ctx, []string{"ResourcePool"}, []string{"summary"}, &rps)
	if err != nil {
		return err
	}

	for _, rp := range rps {
		rpName := rp.Name
		ms := c.entity.NewMetricSet(nrEventType)
		err := ms.SetMetric("name", rpName, metric.ATTRIBUTE)
		if err != nil {
			log.Error(err.Error())
		}
	}
	return nil
}
