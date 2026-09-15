package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	RequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	
	CacheHitRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_hit_rate",
			Help: "Cache hit rate",
		},
		[]string{"cache_type"},
	)
	
	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	
	AICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_call_duration_seconds",
			Help:    "AI API call duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"model"},
	)
	
	CrawlerSuccessRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "crawler_success_rate",
			Help: "Crawler success rate",
		},
		[]string{"target"},
	)
)

func Init() {
	prometheus.MustRegister(RequestCount)
	prometheus.MustRegister(RequestDuration)
	prometheus.MustRegister(CacheHitRate)
	prometheus.MustRegister(DBQueryDuration)
	prometheus.MustRegister(AICallDuration)
	prometheus.MustRegister(CrawlerSuccessRate)
}

func StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":"+port, nil)
}
