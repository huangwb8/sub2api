package usagestats

import (
	"context"
	"sync"
	"time"
)

type AccountUsageStatsInclude uint8

const (
	AccountUsageStatsIncludeNone    AccountUsageStatsInclude = 0
	AccountUsageStatsIncludeSummary AccountUsageStatsInclude = 1 << iota
	AccountUsageStatsIncludeHistory
	AccountUsageStatsIncludeModels
	AccountUsageStatsIncludeEndpoints
	AccountUsageStatsIncludeUpstreamEndpoints

	AccountUsageStatsIncludeAll = AccountUsageStatsIncludeSummary |
		AccountUsageStatsIncludeHistory |
		AccountUsageStatsIncludeModels |
		AccountUsageStatsIncludeEndpoints |
		AccountUsageStatsIncludeUpstreamEndpoints
)

func (i AccountUsageStatsInclude) Has(flag AccountUsageStatsInclude) bool {
	return i&flag != 0
}

type AccountUsageStatsDetailsResponse struct {
	History           []AccountUsageHistory `json:"history,omitempty"`
	Models            []ModelStat           `json:"models,omitempty"`
	Endpoints         []EndpointStat        `json:"endpoints,omitempty"`
	UpstreamEndpoints []EndpointStat        `json:"upstream_endpoints,omitempty"`
}

type AccountUsageStatsPartialResponse struct {
	Summary           *AccountUsageSummary  `json:"summary,omitempty"`
	History           []AccountUsageHistory `json:"history,omitempty"`
	Models            []ModelStat           `json:"models,omitempty"`
	Endpoints         []EndpointStat        `json:"endpoints,omitempty"`
	UpstreamEndpoints []EndpointStat        `json:"upstream_endpoints,omitempty"`
}

type AccountUsageStatsSummaryResponse struct {
	Summary *AccountUsageSummary `json:"summary,omitempty"`
}

type AccountUsageStatsQueryMetrics struct {
	mu                      sync.Mutex
	HistoryQueryMs          int64 `json:"history_query_ms"`
	AvgDurationQueryMs      int64 `json:"avg_duration_query_ms"`
	ModelStatsQueryMs       int64 `json:"model_stats_query_ms"`
	EndpointStatsQueryMs    int64 `json:"endpoint_stats_query_ms"`
	UpstreamEndpointQueryMs int64 `json:"upstream_endpoint_stats_query_ms"`
}

func (m *AccountUsageStatsQueryMetrics) RecordHistoryQuery(duration time.Duration) {
	m.record(func() {
		m.HistoryQueryMs = duration.Milliseconds()
	})
}

func (m *AccountUsageStatsQueryMetrics) RecordAvgDurationQuery(duration time.Duration) {
	m.record(func() {
		m.AvgDurationQueryMs = duration.Milliseconds()
	})
}

func (m *AccountUsageStatsQueryMetrics) RecordModelStatsQuery(duration time.Duration) {
	m.record(func() {
		m.ModelStatsQueryMs = duration.Milliseconds()
	})
}

func (m *AccountUsageStatsQueryMetrics) RecordEndpointStatsQuery(duration time.Duration) {
	m.record(func() {
		m.EndpointStatsQueryMs = duration.Milliseconds()
	})
}

func (m *AccountUsageStatsQueryMetrics) RecordUpstreamEndpointQuery(duration time.Duration) {
	m.record(func() {
		m.UpstreamEndpointQueryMs = duration.Milliseconds()
	})
}

func (m *AccountUsageStatsQueryMetrics) Snapshot() AccountUsageStatsQueryMetrics {
	if m == nil {
		return AccountUsageStatsQueryMetrics{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return AccountUsageStatsQueryMetrics{
		HistoryQueryMs:          m.HistoryQueryMs,
		AvgDurationQueryMs:      m.AvgDurationQueryMs,
		ModelStatsQueryMs:       m.ModelStatsQueryMs,
		EndpointStatsQueryMs:    m.EndpointStatsQueryMs,
		UpstreamEndpointQueryMs: m.UpstreamEndpointQueryMs,
	}
}

func (m *AccountUsageStatsQueryMetrics) record(update func()) {
	if m == nil || update == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	update()
}

type accountUsageStatsMetricsContextKey struct{}

func WithAccountUsageStatsMetrics(ctx context.Context, metrics *AccountUsageStatsQueryMetrics) context.Context {
	if metrics == nil {
		return ctx
	}
	return context.WithValue(ctx, accountUsageStatsMetricsContextKey{}, metrics)
}

func AccountUsageStatsMetricsFromContext(ctx context.Context) *AccountUsageStatsQueryMetrics {
	if ctx == nil {
		return nil
	}
	metrics, _ := ctx.Value(accountUsageStatsMetricsContextKey{}).(*AccountUsageStatsQueryMetrics)
	return metrics
}
