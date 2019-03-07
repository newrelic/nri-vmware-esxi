package main

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
)

func getHostSystems(ctx context.Context, finder *find.Finder) ([]managedEntity, error) {
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		fmt.Printf("Failed to retrieve %s list: %v\n", "Host System", err)
		return nil, err
	}
	var hostsInstances []managedEntity
	for _, host := range hosts {
		hostsInstances = append(hostsInstances, host)
	}
	return hostsInstances, nil
}

func getVMs(ctx context.Context, finder *find.Finder) ([]managedEntity, error) {
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		fmt.Printf("Failed to retrieve %s list: %v\n", "Virtual Machine", err)
		return nil, err
	}
	var vmInstances []managedEntity
	for _, vm := range vms {
		vmInstances = append(vmInstances, vm)
	}
	return vmInstances, nil
}

func getResourcePools(ctx context.Context, finder *find.Finder) ([]managedEntity, error) {
	rpools, err := finder.ResourcePoolList(ctx, "*")
	if err != nil {
		fmt.Printf("Failed to retrieve %s list: %v\n", "Resource Pool", err)
		return nil, err
	}
	var rpoolInstances []managedEntity
	for _, rpool := range rpools {
		rpoolInstances = append(rpoolInstances, rpool)
	}
	return rpoolInstances, nil
}

func getDatastores(ctx context.Context, finder *find.Finder) ([]managedEntity, error) {
	dss, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		fmt.Printf("Failed to retrieve %s list: %v\n", "Datastore", err)
		return nil, err
	}
	var dsInstances []managedEntity
	for _, ds := range dss {
		dsInstances = append(dsInstances, ds)
	}
	return dsInstances, nil
}
