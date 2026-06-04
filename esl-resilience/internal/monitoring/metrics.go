package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func (em *EnhancedMonitor) initCallMetrics() {
	em.callMetrics = &CallMetrics{
		totalCalls: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calls_total",
			Help: "Total number of calls",
		}),
		activeCalls: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "calls_active",
			Help: "Number of active calls",
		}),
		successfulCalls: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calls_successful_total",
			Help: "Total number of successful calls",
		}),
		failedCalls: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calls_failed_total",
			Help: "Total number of failed calls",
		}),
		callDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "call_duration_seconds",
			Help:    "Call duration in seconds",
			Buckets: []float64{1, 5, 15, 30, 60, 300, 600},
		}),
		callSetupTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "call_setup_time_seconds",
			Help:    "Call setup time in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		}),
		callsByTenant: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "calls_by_tenant_total",
			Help: "Total calls per tenant",
		}, []string{"tenant_id"}),
		callsByHangupCause: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "calls_by_hangup_cause_total",
			Help: "Total calls per hangup cause",
		}, []string{"cause"}),
	}
}

func (em *EnhancedMonitor) initPerformanceMetrics() {
	em.performanceMetrics = &PerformanceMetrics{
		cpuUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "CPU usage percentage",
		}),
		memoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),
		goroutineCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "goroutines_count",
			Help: "Number of goroutines",
		}),
		gcDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "gc_duration_seconds",
			Help:    "Garbage collection duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 1, 10},
		}),
		dbConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		}),
		dbQueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 1, 5, 10},
		}),
		eslEventLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "esl_event_latency_seconds",
			Help:    "ESL event processing latency in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 1},
		}),
	}
}

func (em *EnhancedMonitor) initTenantMetrics() {
	em.tenantMetrics = &TenantMetrics{
		activeTenants: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "tenants_active",
			Help: "Number of active tenants",
		}),
		tenantCallVolume: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tenant_call_volume_total",
			Help: "Call volume per tenant",
		}, []string{"tenant_id"}),
		tenantErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tenant_errors_total",
			Help: "Errors per tenant and type",
		}, []string{"tenant_id", "error_type"}),
		tenantResourceUsage: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "tenant_resource_usage",
			Help: "Resource usage per tenant and type",
		}, []string{"tenant_id", "resource_type"}),
	}
}

func (em *EnhancedMonitor) initSLAMetrics() {
	em.slaMetrics = &SLAMetrics{
		systemUptime: promauto.NewCounter(prometheus.CounterOpts{
			Name: "system_uptime_seconds",
			Help: "System uptime in seconds",
		}),
		callSuccessRate: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "call_success_rate_percent",
			Help: "Call success rate percentage",
		}),
		averageCallDuration: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "average_call_duration_seconds",
			Help: "Average call duration in seconds",
		}),
		availability: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "availability_percent",
			Help: "Service availability percentage",
		}),
		errorRate: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "error_rate_percent",
			Help: "Error rate percentage",
		}),
	}
}
