package health

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds custom Prometheus metrics for the RetroDash bridge server.
type Metrics struct {
	registry      *prometheus.Registry
	memoryBytes   prometheus.Gauge
	activeStreams prometheus.Gauge
	browserReady  prometheus.Gauge
	httpRequests  *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance with all metrics registered on a
// fresh registry. Using a custom registry (instead of the global default)
// ensures test isolation and avoids duplicate registration panics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	memoryBytes := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "retrodash_process_memory_bytes",
		Help: "Total bytes of memory obtained from the OS by the Go runtime (runtime.MemStats.Sys).",
	})

	activeStreams := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "retrodash_active_streams",
		Help: "Current number of connected MJPEG stream clients.",
	})

	browserReady := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "retrodash_browser_ready",
		Help: "Whether the headless browser is ready (1=ready, 0=not ready).",
	})

	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "retrodash_http_requests_total",
		Help: "Total number of HTTP requests handled, partitioned by method, path, and status.",
	}, []string{"method", "path", "status"})

	reg.MustRegister(memoryBytes, activeStreams, browserReady, httpRequests)
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	return &Metrics{
		registry:      reg,
		memoryBytes:   memoryBytes,
		activeStreams: activeStreams,
		browserReady:  browserReady,
		httpRequests:  httpRequests,
	}
}

// Registry returns the underlying Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// UpdateMemory reads the current Go runtime memory stats and updates the gauge.
func (m *Metrics) UpdateMemory() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.memoryBytes.Set(float64(ms.Sys))
}

// SetActiveStreams sets the active MJPEG stream client count.
func (m *Metrics) SetActiveStreams(n int) {
	m.activeStreams.Set(float64(n))
}

// SetBrowserReady sets the browser-ready gauge to 1 when ready, 0 otherwise.
func (m *Metrics) SetBrowserReady(ready bool) {
	if ready {
		m.browserReady.Set(1)
	} else {
		m.browserReady.Set(0)
	}
}

// IncHTTPRequests increments the HTTP request counter for the given
// method, path, and HTTP status string (e.g. "200").
func (m *Metrics) IncHTTPRequests(method, path, status string) {
	m.httpRequests.WithLabelValues(method, path, status).Inc()
}
