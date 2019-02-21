package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/jeremywohl/flatten"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

type EventListener struct {
	ctx         context.Context
	client      *govmomi.Client
	integration *integration.Integration
}

func NewEventListener(ctx context.Context, client *govmomi.Client, integration *integration.Integration) *EventListener {
	return &EventListener{
		ctx:         ctx,
		client:      client,
		integration: integration,
	}
}

func (e *EventListener) createEventListeners() error {

	all := true
	finder := find.NewFinder(e.client.Client, all)

	if vmDatacenter == "default" {
		// Find one and only datacenter
		dc, err := finder.DefaultDatacenter(e.ctx)
		if err != nil {
			return err
		}
		e.createEventListenerForDatacenter(finder, dc)
	} else if vmDatacenter == "all" {
		dcList, err := finder.DatacenterList(e.ctx, "*")
		if err != nil {
			return err
		}
		for i, dcItem := range dcList {
			if i > len(dcList) {
				go e.createEventListenerForDatacenter(finder, dcItem)
			} else {
				e.createEventListenerForDatacenter(finder, dcItem)
			}
		}
	} else {
		dc, err := finder.Datacenter(e.ctx, vmDatacenter)
		if err != nil {
			return err
		}
		e.createEventListenerForDatacenter(finder, dc)
	}
	return nil
}

func (e *EventListener) createEventListenerForDatacenter(finder *find.Finder, dc *object.Datacenter) error {
	log.Debug("Creating event listener for datacenter [%s]", dc.Name())

	refs := []types.ManagedObjectReference{dc.Reference()}

	eventManager := event.NewManager(e.client.Client)
	err := eventManager.Events(e.ctx, refs, 10, true, false, e.handleEvent)
	if err != nil {
		return err
	}

	log.Debug("Successfully created event listener for datacenter [%s]", dc.Name())

	return nil
}

func (e *EventListener) handleEvent(ref types.ManagedObjectReference, events []types.BaseEvent) (err error) {
	for _, event := range events {
		eventJson, _ := json.Marshal(event)
		flatEventJson, _ := flatten.FlattenString(string(eventJson), "", flatten.DotStyle)
		log.Debug("Event Detected! Details: %s", flatEventJson)
		var flatEvent map[string]interface{}
		json.Unmarshal([]byte(flatEventJson), &flatEvent)
		entity, err := e.integration.Entity("datacenter", flatEvent["Datacenter.Name"].(string))
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		ms := entity.NewMetricSet("ESXEventListenerSample")
		ESXEventType := strings.Split(reflect.TypeOf(event).String(), ".")
		err = ms.SetMetric("ESXEventType", ESXEventType[len(ESXEventType)-1], metric.ATTRIBUTE)
		if err != nil {
			log.Error(err.Error())
		}
		for key, value := range flatEvent {
			if value != nil {
				switch value.(type) {
				case string:
					timeParsed, err := time.Parse("2006-01-02T15:04:05.999999Z", value.(string))
					if err == nil {
						err = ms.SetMetric(key, timeParsed.Unix(), metric.GAUGE)
						if err != nil {
							log.Error(err.Error())
						}
					} else {
						err = ms.SetMetric(key, value, metric.ATTRIBUTE)
						if err != nil {
							log.Error(err.Error())
						}
					}
				default:
					err = ms.SetMetric(key, value, metric.GAUGE)
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		}
	}
	if err := e.integration.Publish(); err != nil {
		log.Error(err.Error())
	}

	return nil
}
