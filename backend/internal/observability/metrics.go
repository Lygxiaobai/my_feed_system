package observability

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	CacheVideoDetail = "video_detail"
	CacheFeedLatest  = "feed_latest"
	CacheFeedHot     = "feed_hot"
)

var (
	registerMetricsOnce sync.Once

	cacheL1HitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_l1_hit_total",
			Help: "Total number of cache L1 hits.",
		},
		[]string{"cache"},
	)
	cacheL1MissTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_l1_miss_total",
			Help: "Total number of cache L1 misses.",
		},
		[]string{"cache"},
	)
	cacheL1EvictTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_l1_evict_total",
			Help: "Total number of cache L1 evictions.",
		},
		[]string{"cache"},
	)
	cacheL2HitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_l2_hit_total",
			Help: "Total number of cache L2 hits.",
		},
		[]string{"cache"},
	)
	cacheL2MissTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_l2_miss_total",
			Help: "Total number of cache L2 misses.",
		},
		[]string{"cache"},
	)
	cacheSFSharedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_sf_shared_total",
			Help: "Total number of shared singleflight loads.",
		},
		[]string{"cache"},
	)
	cacheInvalidationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_invalidation_total",
			Help: "Total number of cache invalidations.",
		},
		[]string{"cache", "target", "source"},
	)
	cacheLoadDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_load_duration_seconds",
			Help:    "Latency of cache misses loaded from source of truth.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"cache"},
	)

	// 以下两个是 RED 指标（Rate / Errors / Duration），
	// 用于回答「现在每秒多少请求、多少在报错、慢到什么程度」这三个最基本的问题。
	//
	// route 标签取路由模板（/video/:id）而非真实路径，否则每个视频 ID 都会生成
	// 一条独立时间序列，很快把 Prometheus 的内存和磁盘撑爆。
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, route and status.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request latency in seconds.",
			// 默认桶上限只有 10s，但本服务大部分接口在毫秒级，
			// 这里下探到 5ms 以便算出有意义的 P95/P99。
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route"},
	)
)

func NewMetricsHandler() http.Handler {
	registerMetrics()
	return promhttp.Handler()
}

func IncCacheL1Hit(cacheName string) {
	registerMetrics()
	cacheL1HitTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheL1Miss(cacheName string) {
	registerMetrics()
	cacheL1MissTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheL1Evict(cacheName string) {
	registerMetrics()
	cacheL1EvictTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheL2Hit(cacheName string) {
	registerMetrics()
	cacheL2HitTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheL2Miss(cacheName string) {
	registerMetrics()
	cacheL2MissTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheSingleflightShared(cacheName string) {
	registerMetrics()
	cacheSFSharedTotal.WithLabelValues(cacheName).Inc()
}

func IncCacheInvalidation(cacheName string, target string, source string) {
	registerMetrics()
	cacheInvalidationTotal.WithLabelValues(cacheName, target, source).Inc()
}

func ObserveCacheLoadSeconds(cacheName string, seconds float64) {
	registerMetrics()
	cacheLoadDurationSeconds.WithLabelValues(cacheName).Observe(seconds)
}

// ObserveHTTPRequest 记录一次 HTTP 请求的计数与耗时。
// 由访问日志中间件在请求结束时调用，与日志共用同一份现成数据，不额外增加开销。
func ObserveHTTPRequest(method string, route string, status int, seconds float64) {
	registerMetrics()
	httpRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, route).Observe(seconds)
}

func registerMetrics() {
	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(
			cacheL1HitTotal,
			cacheL1MissTotal,
			cacheL1EvictTotal,
			cacheL2HitTotal,
			cacheL2MissTotal,
			cacheSFSharedTotal,
			cacheInvalidationTotal,
			cacheLoadDurationSeconds,
			httpRequestsTotal,
			httpRequestDurationSeconds,
		)
	})
}
