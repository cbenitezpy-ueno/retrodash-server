package health

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	require.NotNil(t, m)
	assert.NotNil(t, m.registry)
	assert.NotNil(t, m.memoryBytes)
	assert.NotNil(t, m.activeStreams)
	assert.NotNil(t, m.browserReady)
	assert.NotNil(t, m.httpRequests)
}

func TestRegistry(t *testing.T) {
	m := NewMetrics()
	reg := m.Registry()
	require.NotNil(t, reg)
	// Registry returned must be the same instance stored internally.
	assert.Equal(t, m.registry, reg)
}

func TestNewMetrics_IncludesGoAndProcessCollectors(t *testing.T) {
	m := NewMetrics()
	// Gather all metrics and check for Go runtime metrics
	families, err := m.Registry().Gather()
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}
	// Go collector registers go_goroutines among others
	assert.True(t, names["go_goroutines"], "expected go_goroutines from GoCollector")
}

func TestUpdateMemory(t *testing.T) {
	m := NewMetrics()
	m.UpdateMemory()

	value := testutil.ToFloat64(m.memoryBytes)
	assert.Greater(t, value, float64(0), "memory gauge should be positive after UpdateMemory")
}

func TestSetActiveStreams(t *testing.T) {
	m := NewMetrics()

	m.SetActiveStreams(7)
	assert.Equal(t, float64(7), testutil.ToFloat64(m.activeStreams))

	m.SetActiveStreams(0)
	assert.Equal(t, float64(0), testutil.ToFloat64(m.activeStreams))
}

func TestSetBrowserReady(t *testing.T) {
	m := NewMetrics()

	m.SetBrowserReady(true)
	assert.Equal(t, float64(1), testutil.ToFloat64(m.browserReady))

	m.SetBrowserReady(false)
	assert.Equal(t, float64(0), testutil.ToFloat64(m.browserReady))
}

func TestIncHTTPRequests(t *testing.T) {
	m := NewMetrics()

	m.IncHTTPRequests("GET", "/health", "200")
	m.IncHTTPRequests("GET", "/health", "200")
	m.IncHTTPRequests("POST", "/stream", "500")

	getHealth := testutil.ToFloat64(m.httpRequests.WithLabelValues("GET", "/health", "200"))
	assert.Equal(t, float64(2), getHealth)

	postStream := testutil.ToFloat64(m.httpRequests.WithLabelValues("POST", "/stream", "500"))
	assert.Equal(t, float64(1), postStream)
}
