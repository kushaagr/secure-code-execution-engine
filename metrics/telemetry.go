package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Track the total processing duration split by language runtime profiles
	ExecutionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "engine_execution_latency_seconds",
		Help:    "Duration of code execution inside the isolated sandbox engine runtimes.",
		Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 7.0},
	}, []string{"language"})

	// Count the frequency of security context timeout cancellations (e.g., infinite loop attacks)
	TimeoutExceededCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "engine_sandbox_timeouts_total",
		Help: "Total count of scripts forcefully killed due to exceeding the 5-second execution ceiling.",
	})

	// Monitor the active workload distribution inside our worker routine routines
	ActiveWorkersGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "engine_active_workers_count",
		Help: "Current count of worker threads actively pulling and processing code footprints.",
	})
)
