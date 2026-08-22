package readmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Class string

const (
	ClassSingleton Class = "singleton"
	ClassDirectory Class = "directory"
	ClassDetail    Class = "detail"
	ClassPage      Class = "page"
	ClassAggregate Class = "aggregate"
	ClassTimeline  Class = "timeline"
	ClassStream    Class = "stream"
)

type Strategy string

const (
	StrategyCurrent          Strategy = "current"
	StrategyCachedDirectory  Strategy = "cached_directory"
	StrategyBoundedDetail    Strategy = "bounded_detail"
	StrategyKeyset           Strategy = "keyset"
	StrategyGenerationKeyset Strategy = "generation_keyset"
	StrategyProjection       Strategy = "projection"
	StrategyTopK             Strategy = "indexed_top_k"
	StrategyStream           Strategy = "bounded_stream"
)

type Maturity string

const (
	MaturityVerified Maturity = "verified"
	MaturityBounded  Maturity = "bounded"
	MaturityPending  Maturity = "migration_pending"
)

type Contract struct {
	ID                 string   `json:"id"`
	Class              Class    `json:"class"`
	TargetStrategy     Strategy `json:"target_strategy"`
	Maturity           Maturity `json:"maturity"`
	Surfaces           []string `json:"surfaces"`
	DefaultRows        int      `json:"default_rows"`
	MaxRows            int      `json:"max_rows"`
	MaxNestedRows      int      `json:"max_nested_rows"`
	MaxResponseBytes   int64    `json:"max_response_bytes"`
	TargetQueryP95MS   int64    `json:"target_query_p95_ms"`
	TargetHandlerP95MS int64    `json:"target_handler_p95_ms"`
	Snapshot           string   `json:"snapshot"`
}

type Declaration struct {
	Route    string   `json:"route"`
	Contract Contract `json:"contract"`
}

type Inventory struct {
	Total            int           `json:"total"`
	Verified         int           `json:"verified"`
	Bounded          int           `json:"bounded"`
	MigrationPending int           `json:"migration_pending"`
	Declarations     []Declaration `json:"declarations,omitempty"`
}

func (contract Contract) Validate() error {
	if strings.TrimSpace(contract.ID) == "" {
		return fmt.Errorf("id is required")
	}
	switch contract.Class {
	case ClassSingleton, ClassDirectory, ClassDetail, ClassPage, ClassAggregate, ClassTimeline, ClassStream:
	default:
		return fmt.Errorf("unknown class %q", contract.Class)
	}
	switch contract.TargetStrategy {
	case StrategyCurrent, StrategyCachedDirectory, StrategyBoundedDetail, StrategyKeyset,
		StrategyGenerationKeyset, StrategyProjection, StrategyTopK, StrategyStream:
	default:
		return fmt.Errorf("unknown target strategy %q", contract.TargetStrategy)
	}
	switch contract.Maturity {
	case MaturityVerified, MaturityBounded, MaturityPending:
	default:
		return fmt.Errorf("unknown maturity %q", contract.Maturity)
	}
	if len(contract.Surfaces) == 0 {
		return fmt.Errorf("at least one page surface is required")
	}
	if contract.MaxRows <= 0 {
		return fmt.Errorf("max rows must be positive")
	}
	if contract.DefaultRows <= 0 || contract.DefaultRows > contract.MaxRows {
		return fmt.Errorf("default rows must be between 1 and max rows")
	}
	if contract.MaxResponseBytes <= 0 {
		return fmt.Errorf("max response bytes must be positive")
	}
	if contract.TargetQueryP95MS <= 0 || contract.TargetHandlerP95MS <= 0 {
		return fmt.Errorf("query and handler p95 targets must be positive")
	}
	if strings.TrimSpace(contract.Snapshot) == "" {
		return fmt.Errorf("snapshot rule is required")
	}
	if contract.Class == ClassSingleton && contract.MaxRows != 1 {
		return fmt.Errorf("singleton must return exactly one row")
	}
	if (contract.Class == ClassPage || contract.Class == ClassTimeline) && contract.MaxRows > 200 {
		return fmt.Errorf("page and timeline contracts cannot exceed 200 rows")
	}
	if contract.Class == ClassStream && contract.TargetStrategy != StrategyStream {
		return fmt.Errorf("stream contract requires bounded stream strategy")
	}
	if contract.Class == ClassDetail && contract.TargetStrategy != StrategyBoundedDetail {
		return fmt.Errorf("detail contract requires bounded detail strategy")
	}
	return nil
}

type Observation struct {
	ContractID         string        `json:"contract_id"`
	Route              string        `json:"route"`
	Class              Class         `json:"class"`
	TargetStrategy     Strategy      `json:"target_strategy"`
	Maturity           Maturity      `json:"maturity"`
	Requests           uint64        `json:"requests"`
	Errors             uint64        `json:"errors"`
	P50                time.Duration `json:"-"`
	P95                time.Duration `json:"-"`
	P99                time.Duration `json:"-"`
	Max                time.Duration `json:"-"`
	MaxBytes           int64         `json:"max_bytes"`
	TotalQueries       uint64        `json:"total_queries"`
	QueryErrors        uint64        `json:"query_errors"`
	MaxQueries         uint64        `json:"max_queries_per_request"`
	MaxRows            int64         `json:"max_rows_per_request"`
	QueryP95           time.Duration `json:"-"`
	QueryMax           time.Duration `json:"-"`
	TargetQueryP95MS   int64         `json:"target_query_p95_ms"`
	TargetHandlerP95MS int64         `json:"target_handler_p95_ms"`
	HandlerBreaches    uint64        `json:"handler_budget_breaches"`
	ResponseBreaches   uint64        `json:"response_budget_breaches"`
	QueryBreaches      uint64        `json:"query_budget_breaches"`
	LastStatus         int           `json:"last_status"`
	LastObservedAt     time.Time     `json:"last_observed_at"`
}

func (observation Observation) MarshalJSON() ([]byte, error) {
	type wire struct {
		ContractID       string   `json:"contract_id"`
		Route            string   `json:"route"`
		Class            Class    `json:"class"`
		TargetStrategy   Strategy `json:"target_strategy"`
		Maturity         Maturity `json:"maturity"`
		Requests         uint64   `json:"requests"`
		Errors           uint64   `json:"errors"`
		P50MS            float64  `json:"p50_ms"`
		P95MS            float64  `json:"p95_ms"`
		P99MS            float64  `json:"p99_ms"`
		MaxMS            float64  `json:"max_ms"`
		MaxBytes         int64    `json:"max_bytes"`
		TotalQueries     uint64   `json:"total_queries"`
		QueryErrors      uint64   `json:"query_errors"`
		MaxQueries       uint64   `json:"max_queries_per_request"`
		MaxRows          int64    `json:"max_rows_per_request"`
		QueryP95MS       float64  `json:"query_p95_ms"`
		QueryMaxMS       float64  `json:"query_max_ms"`
		HandlerTargetMS  int64    `json:"handler_target_p95_ms"`
		QueryTargetMS    int64    `json:"query_target_p95_ms"`
		HandlerBreaches  uint64   `json:"handler_budget_breaches"`
		ResponseBreaches uint64   `json:"response_budget_breaches"`
		QueryBreaches    uint64   `json:"query_budget_breaches"`
		LastStatus       int      `json:"last_status"`
		LastObservedAt   string   `json:"last_observed_at"`
	}
	return json.Marshal(wire{
		ContractID: observation.ContractID, Route: observation.Route,
		Class: observation.Class, TargetStrategy: observation.TargetStrategy,
		Maturity: observation.Maturity, Requests: observation.Requests, Errors: observation.Errors,
		P50MS: durationMS(observation.P50), P95MS: durationMS(observation.P95),
		P99MS: durationMS(observation.P99), MaxMS: durationMS(observation.Max),
		MaxBytes: observation.MaxBytes, TotalQueries: observation.TotalQueries,
		QueryErrors: observation.QueryErrors, MaxQueries: observation.MaxQueries,
		MaxRows: observation.MaxRows, QueryP95MS: durationMS(observation.QueryP95),
		QueryMaxMS:       durationMS(observation.QueryMax),
		HandlerTargetMS:  observation.TargetHandlerP95MS,
		QueryTargetMS:    observation.TargetQueryP95MS,
		HandlerBreaches:  observation.HandlerBreaches,
		ResponseBreaches: observation.ResponseBreaches,
		QueryBreaches:    observation.QueryBreaches,
		LastStatus:       observation.LastStatus,
		LastObservedAt:   observation.LastObservedAt.UTC().Format(time.RFC3339Nano),
	})
}

type routeMetric struct {
	contract         Contract
	requests         uint64
	errors           uint64
	durations        []time.Duration
	maxDuration      time.Duration
	maxBytes         int64
	totalQueries     uint64
	queryErrors      uint64
	maxQueries       uint64
	maxRows          int64
	queryDurations   []time.Duration
	queryMax         time.Duration
	handlerBreaches  uint64
	responseBreaches uint64
	queryBreaches    uint64
	lastStatus       int
	lastObservedAt   time.Time
}

type requestMetricsContextKey struct{}

type requestMetrics struct {
	mu             sync.Mutex
	queries        uint64
	queryErrors    uint64
	rows           int64
	maxQuery       time.Duration
	queryDurations []time.Duration
}

type requestMetricSnapshot struct {
	queries        uint64
	queryErrors    uint64
	rows           int64
	maxQuery       time.Duration
	queryDurations []time.Duration
}

func withRequestMetrics(ctx context.Context) (context.Context, *requestMetrics) {
	metrics := &requestMetrics{queryDurations: make([]time.Duration, 0, 8)}
	return context.WithValue(ctx, requestMetricsContextKey{}, metrics), metrics
}

func metricsFromContext(ctx context.Context) *requestMetrics {
	if ctx == nil {
		return nil
	}
	metrics, _ := ctx.Value(requestMetricsContextKey{}).(*requestMetrics)
	return metrics
}

func (metrics *requestMetrics) record(duration time.Duration, rows int64, queryErr error) {
	if metrics == nil {
		return
	}
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	metrics.queries++
	if queryErr != nil {
		metrics.queryErrors++
	}
	if rows > 0 {
		metrics.rows += rows
	}
	if duration > metrics.maxQuery {
		metrics.maxQuery = duration
	}
	metrics.queryDurations = append(metrics.queryDurations, duration)
}

func (metrics *requestMetrics) snapshot() requestMetricSnapshot {
	if metrics == nil {
		return requestMetricSnapshot{}
	}
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	return requestMetricSnapshot{
		queries: metrics.queries, queryErrors: metrics.queryErrors, rows: metrics.rows,
		maxQuery: metrics.maxQuery, queryDurations: append([]time.Duration(nil), metrics.queryDurations...),
	}
}

type Registry struct {
	mu        sync.RWMutex
	contracts map[string]Contract
	metrics   map[string]*routeMetric
	order     []string
}

const observationWindow = 2048

func NewRegistry(contracts map[string]Contract) (*Registry, error) {
	registry := &Registry{
		contracts: make(map[string]Contract, len(contracts)),
		metrics:   make(map[string]*routeMetric, len(contracts)),
		order:     make([]string, 0, len(contracts)),
	}
	for route, contract := range contracts {
		if !strings.HasPrefix(route, "/api/") && route != "/api/status" {
			return nil, fmt.Errorf("read contract route must be an API path: %s", route)
		}
		if err := contract.Validate(); err != nil {
			return nil, fmt.Errorf("read contract %s: %w", route, err)
		}
		contract.Surfaces = append([]string(nil), contract.Surfaces...)
		registry.contracts[route] = contract
		registry.metrics[route] = &routeMetric{contract: contract, durations: make([]time.Duration, 0, 64)}
		registry.order = append(registry.order, route)
	}
	sort.Strings(registry.order)
	return registry, nil
}

func (registry *Registry) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if registry == nil || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		route, contract, ok := registry.match(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Well-Ambient-Read-Contract", contract.ID)
		w.Header().Set("X-Well-Ambient-Read-Class", string(contract.Class))
		w.Header().Set("X-Well-Ambient-Read-Target-Strategy", string(contract.TargetStrategy))
		w.Header().Set("X-Well-Ambient-Read-Maturity", string(contract.Maturity))
		writer := &countingWriter{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		ctx, requestMetrics := withRequestMetrics(r.Context())
		next.ServeHTTP(writer, r.WithContext(ctx))
		duration := time.Since(started)
		registry.observe(route, writer.status, writer.bytes, duration, requestMetrics.snapshot(), time.Now())
		if writer.bytes > contract.MaxResponseBytes {
			log.Printf("read contract response budget exceeded route=%s bytes=%d budget=%d contract=%s", route, writer.bytes, contract.MaxResponseBytes, contract.ID)
		}
	})
}

func (registry *Registry) Snapshot() []Observation {
	if registry == nil {
		return []Observation{}
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	result := make([]Observation, 0, len(registry.metrics))
	for _, route := range registry.order {
		metric := registry.metrics[route]
		if metric == nil || metric.requests == 0 {
			continue
		}
		durations := append([]time.Duration(nil), metric.durations...)
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		queryDurations := append([]time.Duration(nil), metric.queryDurations...)
		sort.Slice(queryDurations, func(i, j int) bool { return queryDurations[i] < queryDurations[j] })
		result = append(result, Observation{
			ContractID: metric.contract.ID, Route: route, Class: metric.contract.Class,
			TargetStrategy: metric.contract.TargetStrategy, Maturity: metric.contract.Maturity,
			Requests: metric.requests, Errors: metric.errors,
			P50: percentile(durations, 0.50), P95: percentile(durations, 0.95),
			P99: percentile(durations, 0.99), Max: metric.maxDuration,
			MaxBytes: metric.maxBytes, TotalQueries: metric.totalQueries,
			QueryErrors: metric.queryErrors, MaxQueries: metric.maxQueries, MaxRows: metric.maxRows,
			QueryP95: percentile(queryDurations, 0.95), QueryMax: metric.queryMax,
			TargetQueryP95MS:   metric.contract.TargetQueryP95MS,
			TargetHandlerP95MS: metric.contract.TargetHandlerP95MS,
			HandlerBreaches:    metric.handlerBreaches, ResponseBreaches: metric.responseBreaches,
			QueryBreaches: metric.queryBreaches, LastStatus: metric.lastStatus,
			LastObservedAt: metric.lastObservedAt,
		})
	}
	return result
}

func (registry *Registry) Inventory(includeDeclarations ...bool) Inventory {
	if registry == nil {
		return Inventory{}
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	include := len(includeDeclarations) > 0 && includeDeclarations[0]
	inventory := Inventory{Total: len(registry.order)}
	if include {
		inventory.Declarations = make([]Declaration, 0, len(registry.order))
	}
	for _, route := range registry.order {
		contract := registry.contracts[route]
		if include {
			contract.Surfaces = append([]string(nil), contract.Surfaces...)
			inventory.Declarations = append(inventory.Declarations, Declaration{Route: route, Contract: contract})
		}
		switch contract.Maturity {
		case MaturityVerified:
			inventory.Verified++
		case MaturityBounded:
			inventory.Bounded++
		case MaturityPending:
			inventory.MigrationPending++
		}
	}
	return inventory
}

func (registry *Registry) match(path string) (string, Contract, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if contract, ok := registry.contracts[path]; ok {
		return path, contract, true
	}
	for _, route := range registry.order {
		if strings.Contains(route, "{") && routeMatches(route, path) {
			return route, registry.contracts[route], true
		}
	}
	return "", Contract{}, false
}

func (registry *Registry) observe(route string, status int, bytes int64, duration time.Duration, request requestMetricSnapshot, observedAt time.Time) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	metric := registry.metrics[route]
	if metric == nil {
		return
	}
	metric.requests++
	if status >= http.StatusBadRequest {
		metric.errors++
	}
	if len(metric.durations) == observationWindow {
		copy(metric.durations, metric.durations[1:])
		metric.durations[len(metric.durations)-1] = duration
	} else {
		metric.durations = append(metric.durations, duration)
	}
	if duration > metric.maxDuration {
		metric.maxDuration = duration
	}
	if bytes > metric.maxBytes {
		metric.maxBytes = bytes
	}
	metric.totalQueries += request.queries
	metric.queryErrors += request.queryErrors
	if request.queries > metric.maxQueries {
		metric.maxQueries = request.queries
	}
	if request.rows > metric.maxRows {
		metric.maxRows = request.rows
	}
	if request.maxQuery > metric.queryMax {
		metric.queryMax = request.maxQuery
	}
	for _, queryDuration := range request.queryDurations {
		if len(metric.queryDurations) == observationWindow {
			copy(metric.queryDurations, metric.queryDurations[1:])
			metric.queryDurations[len(metric.queryDurations)-1] = queryDuration
		} else {
			metric.queryDurations = append(metric.queryDurations, queryDuration)
		}
	}
	if duration > time.Duration(metric.contract.TargetHandlerP95MS)*time.Millisecond {
		metric.handlerBreaches++
	}
	if bytes > metric.contract.MaxResponseBytes {
		metric.responseBreaches++
	}
	if request.maxQuery > time.Duration(metric.contract.TargetQueryP95MS)*time.Millisecond {
		metric.queryBreaches++
	}
	metric.lastStatus = status
	metric.lastObservedAt = observedAt
}

type countingWriter struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (writer *countingWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.wroteHeader = true
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *countingWriter) Write(payload []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := writer.ResponseWriter.Write(payload)
	writer.bytes += int64(written)
	return written, err
}

func (writer *countingWriter) Flush() {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (writer *countingWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func routeMatches(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for index := range patternParts {
		if strings.HasPrefix(patternParts[index], "{") && strings.HasSuffix(patternParts[index], "}") {
			if pathParts[index] == "" {
				return false
			}
			continue
		}
		if patternParts[index] != pathParts[index] {
			return false
		}
	}
	return true
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * quantile)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func durationMS(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000
}
