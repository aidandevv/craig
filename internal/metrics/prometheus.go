package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	Ingested       prometheus.Counter
	Processed      *prometheus.CounterVec
	QueueDepth     prometheus.Gauge
	SignalLatency  *prometheus.HistogramVec
	TotalLatency   prometheus.Histogram
	SignalErrors   *prometheus.CounterVec
	APIErrors      *prometheus.CounterVec
	LastPoll       prometheus.Gauge
	VisionUnits    *prometheus.CounterVec
	VisionSkipped  *prometheus.CounterVec
	FraudScores    prometheus.Histogram
	Notifications  *prometheus.CounterVec
	MonthlyCost    *prometheus.GaugeVec
	RunningCost    prometheus.Gauge
	CostPerListing *prometheus.GaugeVec
}

func New() *Metrics {
	prefix := "apartment_hunter_"
	started := time.Now()
	promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: prefix + "service_uptime_seconds", Help: "Seconds since this Go service started."}, func() float64 { return time.Since(started).Seconds() })
	return &Metrics{
		Ingested:       promauto.NewCounter(prometheus.CounterOpts{Name: prefix + "listings_ingested_total", Help: "Listings first discovered through Flipify."}),
		Processed:      promauto.NewCounterVec(prometheus.CounterOpts{Name: prefix + "listings_processed_total", Help: "Listings evaluated by outcome."}, []string{"status"}),
		QueueDepth:     promauto.NewGauge(prometheus.GaugeOpts{Name: prefix + "listings_queue_depth", Help: "Listings waiting for processing."}),
		SignalLatency:  promauto.NewHistogramVec(prometheus.HistogramOpts{Name: prefix + "signal_latency_ms", Help: "Fraud signal latency in milliseconds.", Buckets: prometheus.ExponentialBuckets(1, 2, 12)}, []string{"signal"}),
		TotalLatency:   promauto.NewHistogram(prometheus.HistogramOpts{Name: prefix + "total_evaluation_latency_ms", Help: "Total fraud evaluation latency in milliseconds.", Buckets: prometheus.ExponentialBuckets(1, 2, 14)}),
		SignalErrors:   promauto.NewCounterVec(prometheus.CounterOpts{Name: prefix + "signal_errors_total", Help: "Fraud signal errors."}, []string{"signal", "error_type"}),
		APIErrors:      promauto.NewCounterVec(prometheus.CounterOpts{Name: prefix + "api_errors_total", Help: "Provider API errors."}, []string{"service", "status_code"}),
		LastPoll:       promauto.NewGauge(prometheus.GaugeOpts{Name: prefix + "last_flipify_poll_success_timestamp_seconds", Help: "Unix time of the latest successful Flipify search."}),
		VisionUnits:    promauto.NewCounterVec(prometheus.CounterOpts{Name: prefix + "vision_feature_units_total", Help: "Vision units reserved by feature."}, []string{"feature"}),
		VisionSkipped:  promauto.NewCounterVec(prometheus.CounterOpts{Name: prefix + "vision_budget_skipped_total", Help: "Images skipped because the monthly feature cap was reached."}, []string{"feature"}),
		FraudScores:    promauto.NewHistogram(prometheus.HistogramOpts{Name: prefix + "fraud_score", Help: "Fraud-score distribution.", Buckets: prometheus.LinearBuckets(0, .1, 11)}),
		Notifications:  promauto.NewCounterVec(prometheus.CounterOpts{Name: prefix + "notification_deliveries_total", Help: "Notification delivery attempts."}, []string{"channel", "status"}),
		MonthlyCost:    promauto.NewGaugeVec(prometheus.GaugeOpts{Name: prefix + "monthly_cost_total", Help: "Current calendar-month cost by service."}, []string{"service"}),
		RunningCost:    promauto.NewGauge(prometheus.GaugeOpts{Name: prefix + "cost_running_total", Help: "Rolling 30-day operational cost."}),
		CostPerListing: promauto.NewGaugeVec(prometheus.GaugeOpts{Name: prefix + "cost_per_listing", Help: "Rolling 30-day cost per listing outcome."}, []string{"unit"}),
	}
}

func (m *Metrics) SetCosts(monthly map[string]float64, running float64, processed, qualified, notified int) {
	for service, cost := range monthly {
		m.MonthlyCost.WithLabelValues(service).Set(cost)
	}
	m.RunningCost.Set(running)
	m.CostPerListing.WithLabelValues("processed").Set(perUnit(running, processed))
	m.CostPerListing.WithLabelValues("qualified").Set(perUnit(running, qualified))
	m.CostPerListing.WithLabelValues("notified").Set(perUnit(running, notified))
}
func perUnit(cost float64, count int) float64 {
	if count == 0 {
		return 0
	}
	return cost / float64(count)
}
