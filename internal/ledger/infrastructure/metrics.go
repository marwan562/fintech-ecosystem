package infrastructure

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TransactionsRecorded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ledger_transactions_recorded_total",
		Help: "Total number of successfully recorded ledger transactions.",
	}, []string{"status"})

	TransactionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ledger_transaction_latency_seconds",
		Help:    "Latency of ledger transaction recording.",
		Buckets: prometheus.DefBuckets,
	})

	OutboxLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ledger_outbox_lag_total",
		Help: "Current number of unprocessed events in the outbox.",
	})
)

type PrometheusMetrics struct{}

func (m *PrometheusMetrics) RecordTransaction(status string) {
	TransactionsRecorded.WithLabelValues(status).Inc()
}

func (m *PrometheusMetrics) StartTimer() *prometheus.Timer {
	return prometheus.NewTimer(TransactionLatency)
}
