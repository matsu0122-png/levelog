// Package metrics accumulates per-route HTTP request counts and latency
// histograms and renders them in Prometheus text exposition format
// (https://prometheus.io/docs/instrumenting/exposition_formats/). This is a
// hand-rolled exporter rather than client_golang: the backend only needs
// two metrics (request count, request duration), so pulling in the full
// client library and its transitive dependencies isn't worth it here.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// defaultBuckets are HTTP latency boundaries in seconds. Fine-grained
// below 1s, where nearly all requests should land, coarser above it.
var defaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type seriesKey struct {
	method string
	route  string
	status int
}

type histKey struct {
	method string
	route  string
}

// Registry accumulates observations in memory. Safe for concurrent use.
type Registry struct {
	mu      sync.Mutex
	counts  map[seriesKey]int64
	buckets map[histKey][]int64 // parallel to defaultBuckets, cumulative
	sums    map[histKey]float64
	obs     map[histKey]int64
}

func NewRegistry() *Registry {
	return &Registry{
		counts:  make(map[seriesKey]int64),
		buckets: make(map[histKey][]int64),
		sums:    make(map[histKey]float64),
		obs:     make(map[histKey]int64),
	}
}

// Observe records one completed HTTP request. route should be the route's
// registered mux pattern (e.g. "GET /api/missions/{id}"), not the raw URL
// path — the raw path would let path parameters (user/mission IDs) blow up
// the number of distinct label combinations.
func (r *Registry) Observe(method, route string, status int, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counts[seriesKey{method, route, status}]++

	hk := histKey{method, route}
	b, ok := r.buckets[hk]
	if !ok {
		b = make([]int64, len(defaultBuckets))
		r.buckets[hk] = b
	}
	seconds := dur.Seconds()
	for i, upper := range defaultBuckets {
		if seconds <= upper {
			b[i]++
		}
	}
	r.sums[hk] += seconds
	r.obs[hk]++
}

// Handler renders the current metrics in Prometheus text format.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(r.render()))
	})
}

func (r *Registry) render() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var b strings.Builder
	b.WriteString("# HELP levelog_http_requests_total Total HTTP requests processed, by method/route/status.\n")
	b.WriteString("# TYPE levelog_http_requests_total counter\n")
	for _, k := range sortedSeriesKeys(r.counts) {
		fmt.Fprintf(&b, "levelog_http_requests_total{method=%q,route=%q,status=%q} %d\n",
			k.method, k.route, strconv.Itoa(k.status), r.counts[k])
	}

	b.WriteString("# HELP levelog_http_request_duration_seconds HTTP request duration in seconds, by method/route.\n")
	b.WriteString("# TYPE levelog_http_request_duration_seconds histogram\n")
	for _, k := range sortedHistKeys(r.buckets) {
		buckets := r.buckets[k]
		for i, upper := range defaultBuckets {
			fmt.Fprintf(&b, "levelog_http_request_duration_seconds_bucket{method=%q,route=%q,le=%q} %d\n",
				k.method, k.route, formatFloat(upper), buckets[i])
		}
		fmt.Fprintf(&b, "levelog_http_request_duration_seconds_bucket{method=%q,route=%q,le=\"+Inf\"} %d\n",
			k.method, k.route, r.obs[k])
		fmt.Fprintf(&b, "levelog_http_request_duration_seconds_sum{method=%q,route=%q} %s\n",
			k.method, k.route, formatFloat(r.sums[k]))
		fmt.Fprintf(&b, "levelog_http_request_duration_seconds_count{method=%q,route=%q} %d\n",
			k.method, k.route, r.obs[k])
	}
	return b.String()
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func sortedSeriesKeys(m map[seriesKey]int64) []seriesKey {
	keys := make([]seriesKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.method != b.method {
			return a.method < b.method
		}
		if a.route != b.route {
			return a.route < b.route
		}
		return a.status < b.status
	})
	return keys
}

func sortedHistKeys(m map[histKey][]int64) []histKey {
	keys := make([]histKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.method != b.method {
			return a.method < b.method
		}
		return a.route < b.route
	})
	return keys
}
