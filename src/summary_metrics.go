package main

import (
	"context"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

var dsSummary map[string]map[string]interface{}

func populateSummaryMetrics(client *govmomi.Client) error {
	ctx := context.Background()
	dsSummary = make(map[string]map[string]interface{})
	// Create a view of Datastore objects
	manager := view.NewManager(client.Client)

	view, err := manager.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"Datastore", "HostSystem"}, true)
	if err != nil {
		return err
	}

	defer func() {
		if err := view.Destroy(ctx); err != nil {
			log.Error(err.Error())
		}
	}()

	// Retrieve summary property for all datastores
	// Reference: http://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.Datastore.html
	var dss []mo.Datastore
	err = view.Retrieve(ctx, []string{"Datastore"}, []string{"summary"}, &dss)
	if err != nil {
		return err
	}

	// Print summary per datastore (see also: govc/datastore/info.go)

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

		switch info := ds.Info.(type) {
		case *types.NasDatastoreInfo:
			dsMetrics[info.Nas.RemoteHost] = info.Nas.RemotePath
		}
	}
	return nil
}
