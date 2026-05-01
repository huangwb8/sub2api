package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	ProxyProbeSourceScheduled = "scheduled_probe"
	ProxyProbeSourceManual    = "manual_test"
	ProxyProbeTargetChain     = "probe_chain"

	proxyProbeLogErrorMaxLen = 1024
)

type ProxyProbeLog struct {
	ID           int64     `json:"id"`
	ProxyID      int64     `json:"proxy_id"`
	Source       string    `json:"source"`
	Target       string    `json:"target"`
	Success      bool      `json:"success"`
	LatencyMs    *int64    `json:"latency_ms,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	IPAddress    *string   `json:"ip_address,omitempty"`
	CountryCode  *string   `json:"country_code,omitempty"`
	Country      *string   `json:"country,omitempty"`
	Region       *string   `json:"region,omitempty"`
	City         *string   `json:"city,omitempty"`
	CheckedAt    time.Time `json:"checked_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type ProxyProbeLogInput struct {
	ProxyID      int64
	Source       string
	Target       string
	Success      bool
	LatencyMs    *int64
	ErrorMessage string
	ExitInfo     *ProxyExitInfo
	CheckedAt    time.Time
}

type ProxyProbeLogQuery struct {
	ProxyID int64
	Since   time.Time
	Until   time.Time
	Limit   int
	Offset  int
}

type ProxyReliabilityWindow struct {
	Label              string     `json:"label"`
	Hours              int        `json:"hours"`
	ProbeTotal         int64      `json:"probe_total"`
	ProbeSuccess       int64      `json:"probe_success"`
	ProbeSuccessRate   *float64   `json:"probe_success_rate,omitempty"`
	UsageSuccessCount  int64      `json:"usage_success_count"`
	ProxyErrorCount    int64      `json:"proxy_error_count"`
	LastProbeFailureAt *time.Time `json:"last_probe_failure_at,omitempty"`
}

type ProxyReliabilityFollowup struct {
	Minutes           int   `json:"minutes"`
	FailedProbeCount  int64 `json:"failed_probe_count"`
	UsageSuccessCount int64 `json:"usage_success_count"`
	ProxyErrorCount   int64 `json:"proxy_error_count"`
}

type ProxyReliabilityReport struct {
	ProxyID             int64                      `json:"proxy_id"`
	GeneratedAt         time.Time                  `json:"generated_at"`
	BoundAccountCount   int64                      `json:"bound_account_count"`
	LastProbe           *ProxyProbeLog             `json:"last_probe,omitempty"`
	Windows             []ProxyReliabilityWindow   `json:"windows"`
	FailureFollowups    []ProxyReliabilityFollowup `json:"failure_followups"`
	InterpretationNotes []string                   `json:"interpretation_notes"`
}

type ProxyProbeLogRepository interface {
	Create(ctx context.Context, input ProxyProbeLogInput) error
	List(ctx context.Context, query ProxyProbeLogQuery) ([]ProxyProbeLog, *pagination.PaginationResult, error)
	GetLast(ctx context.Context, proxyID int64) (*ProxyProbeLog, error)
	GetReliability(ctx context.Context, proxyID int64, now time.Time) (*ProxyReliabilityReport, error)
	DeleteBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

func NormalizeProxyProbeLogInput(input ProxyProbeLogInput) ProxyProbeLogInput {
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		input.Source = ProxyProbeSourceScheduled
	}
	input.Target = strings.TrimSpace(input.Target)
	if input.Target == "" {
		input.Target = ProxyProbeTargetChain
	}
	input.ErrorMessage = truncateProxyProbeLogMessage(input.ErrorMessage)
	if input.CheckedAt.IsZero() {
		input.CheckedAt = time.Now()
	}
	return input
}

func truncateProxyProbeLogMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= proxyProbeLogErrorMaxLen {
		return message
	}
	return message[:proxyProbeLogErrorMaxLen]
}
