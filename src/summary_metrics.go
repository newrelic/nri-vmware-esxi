package main

import (
	"context"

	"github.com/vmware/govmomi/object"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func collectSummaryMetrics(client *govmomi.Client, dc *object.Datacenter) (map[string]map[string]interface{}, error) {
	ctx := context.Background()
	dsSummary := make(map[string]map[string]interface{})
	// Create a view of Datastore objects
	manager := view.NewManager(client.Client)

	view, err := manager.CreateContainerView(ctx, dc.Reference(), []string{"Datastore"}, true)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	var dss []mo.Datastore
	err = view.Retrieve(ctx, []string{"Datastore"}, []string{"summary"}, &dss)
	if err != nil {
		return nil, err
	}

	for _, ds := range dss {
		dsName := ds.Summary.Name
		dsMetrics, ok := dsSummary[dsName]
		if !ok {
			dsMetrics = make(map[string]interface{})
			dsSummary[dsName] = dsMetrics
		}
		dsMetrics["ds.type"] = ds.Summary.Type
		dsMetrics["ds.url"] = ds.Summary.Url
		dsMetrics["ds.capacity"] = float64(ds.Summary.Capacity) / (1 << 30)
		dsMetrics["ds.freespace"] = float64(ds.Summary.FreeSpace) / (1 << 30)
		dsMetrics["ds.uncommitted"] = float64(ds.Summary.Uncommitted) / (1 << 30)
		dsMetrics["ds.accessible"] = ds.Summary.Accessible

		switch info := ds.Info.(type) {
		case *types.NasDatastoreInfo:
			dsMetrics["ds.nas.remoteHost"] = info.Nas.RemoteHost
			dsMetrics["ds.nas.remotePath"] = info.Nas.RemotePath
		}
	}
	return dsSummary, nil
}
