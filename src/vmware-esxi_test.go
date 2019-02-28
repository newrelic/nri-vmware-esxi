package main

import (
	"testing"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	// Insert here the logic for your tests

	i, err := integration.New("test", "1.0.0")
	assert.NoError(t, err)

	e, err := i.Entity("testEntity", "testNamespace")
	assert.NoError(t, err)

	m := e.NewMetricSet("testMetrics")
	assert.NoError(t, err)

	m.SetMetric("testMetric1", 1, metric.GAUGE)
	m.SetMetric("testMetric2", "foo", metric.ATTRIBUTE)

}
