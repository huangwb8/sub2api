package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	UserRiskControlModeObserveOnly = "observe_only"
	UserRiskControlModeWarnOnly    = "warn_only"
	UserRiskControlModeAutoLock    = "auto_lock"
)

const (
	UserRiskStatusHealthy     = "healthy"
	UserRiskStatusObserved    = "observed"
	UserRiskStatusWarned      = "warned"
	UserRiskStatusGracePeriod = "grace_period"
	UserRiskStatusLocked      = "locked"
	UserRiskStatusExempted    = "exempted"
)

const (
	UserRiskEventTypeDailyEvaluation = "daily_evaluation"
	UserRiskEventTypeOverlap         = "overlap_detected"
	UserRiskEventTypeWarned          = "warning_sent"
	UserRiskEventTypeLocked          = "auto_locked"
	UserRiskEventTypeUnlocked        = "manual_unlock"
	UserRiskEventTypeExemption       = "exemption_changed"
	UserRiskEventTypeScoreReset      = "score_reset"
)

const (
	UserRiskSeverityInfo     = "info"
	UserRiskSeverityWarning  = "warning"
	UserRiskSeverityCritical = "critical"
)

var ErrUserRiskLocked = infraerrors.Forbidden(
	"USER_RISK_LOCKED",
	"Your account has been locked for suspected resale or Terms of Service violation. Please contact the administrator to unlock it.",
)

type UserRiskControlConfig struct {
	Enabled                            bool    `json:"enabled"`
	Mode                               string  `json:"mode"`
	WarningThreshold                   float64 `json:"warning_threshold"`
	LockThreshold                      float64 `json:"lock_threshold"`
	AutoLockAfterConsecutiveBadDays    int     `json:"auto_lock_after_consecutive_bad_days"`
	OverlapWindowSeconds               int     `json:"overlap_window_seconds"`
	MaxDistinctPublicIPsPerDay         int     `json:"max_distinct_public_ips_per_day"`
	HighRiskActiveHoursPerDay          int     `json:"high_risk_active_hours_per_day"`
	WarningEmailEnabled                bool    `json:"warning_email_enabled"`
	WarningEmailSubjectTemplate        string  `json:"warning_email_subject_template"`
	LockMessage                        string  `json:"lock_message"`
	RequireTrustedProxyForAutoLock     bool    `json:"require_trusted_proxy_for_auto_lock"`
	DailyScorePenaltyCap               float64 `json:"daily_score_penalty_cap"`
	DailyScoreRecovery                 float64 `json:"daily_score_recovery"`
	RedisEvidenceRetentionDays         int     `json:"redis_evidence_retention_days"`
}

func DefaultUserRiskControlConfig() *UserRiskControlConfig {
	return &UserRiskControlConfig{
		Enabled:                         false,
		Mode:                            UserRiskControlModeObserveOnly,
		WarningThreshold:                3.0,
		LockThreshold:                   2.0,
		AutoLockAfterConsecutiveBadDays: 3,
		OverlapWindowSeconds:            300,
		MaxDistinctPublicIPsPerDay:      3,
		HighRiskActiveHoursPerDay:       18,
		WarningEmailEnabled:             true,
		WarningEmailSubjectTemplate:     "[Sub2API] Suspicious account activity detected",
		LockMessage:                     ErrUserRiskLocked.Message,
		RequireTrustedProxyForAutoLock:  true,
		DailyScorePenaltyCap:            1.0,
		DailyScoreRecovery:              0.5,
		RedisEvidenceRetentionDays:      7,
	}
}

func NormalizeUserRiskControlConfig(config *UserRiskControlConfig) *UserRiskControlConfig {
	normalized := DefaultUserRiskControlConfig()
	if config == nil {
		return normalized
	}
	*normalized = *config

	normalized.Mode = strings.ToLower(strings.TrimSpace(normalized.Mode))
	if normalized.Mode == "" {
		normalized.Mode = UserRiskControlModeObserveOnly
	}
	if strings.TrimSpace(normalized.WarningEmailSubjectTemplate) == "" {
		normalized.WarningEmailSubjectTemplate = DefaultUserRiskControlConfig().WarningEmailSubjectTemplate
	}
	if strings.TrimSpace(normalized.LockMessage) == "" {
		normalized.LockMessage = ErrUserRiskLocked.Message
	}
	if normalized.OverlapWindowSeconds <= 0 {
		normalized.OverlapWindowSeconds = 300
	}
	if normalized.MaxDistinctPublicIPsPerDay <= 0 {
		normalized.MaxDistinctPublicIPsPerDay = 3
	}
	if normalized.HighRiskActiveHoursPerDay <= 0 {
		normalized.HighRiskActiveHoursPerDay = 18
	}
	if normalized.AutoLockAfterConsecutiveBadDays <= 0 {
		normalized.AutoLockAfterConsecutiveBadDays = 3
	}
	if normalized.DailyScorePenaltyCap <= 0 {
		normalized.DailyScorePenaltyCap = 1.0
	}
	if normalized.DailyScoreRecovery <= 0 {
		normalized.DailyScoreRecovery = 0.5
	}
	if normalized.RedisEvidenceRetentionDays <= 0 {
		normalized.RedisEvidenceRetentionDays = 7
	}
	if normalized.WarningThreshold <= 0 {
		normalized.WarningThreshold = 3.0
	}
	if normalized.LockThreshold <= 0 {
		normalized.LockThreshold = 2.0
	}
	return normalized
}

func ValidateUserRiskControlConfig(config *UserRiskControlConfig) error {
	config = NormalizeUserRiskControlConfig(config)
	switch config.Mode {
	case UserRiskControlModeObserveOnly, UserRiskControlModeWarnOnly, UserRiskControlModeAutoLock:
	default:
		return infraerrors.BadRequest("USER_RISK_CONTROL_MODE_INVALID", "invalid user risk control mode")
	}
	if config.WarningThreshold <= 0 || config.WarningThreshold > 5 {
		return infraerrors.BadRequest("USER_RISK_WARNING_THRESHOLD_INVALID", "warning threshold must be between 0 and 5")
	}
	if config.LockThreshold <= 0 || config.LockThreshold > 5 {
		return infraerrors.BadRequest("USER_RISK_LOCK_THRESHOLD_INVALID", "lock threshold must be between 0 and 5")
	}
	if config.LockThreshold > config.WarningThreshold {
		return infraerrors.BadRequest("USER_RISK_THRESHOLD_ORDER_INVALID", "lock threshold cannot be greater than warning threshold")
	}
	if config.AutoLockAfterConsecutiveBadDays < 1 || config.AutoLockAfterConsecutiveBadDays > 30 {
		return infraerrors.BadRequest("USER_RISK_AUTO_LOCK_DAYS_INVALID", "auto lock days must be between 1 and 30")
	}
	if config.OverlapWindowSeconds < 30 || config.OverlapWindowSeconds > 3600 {
		return infraerrors.BadRequest("USER_RISK_OVERLAP_WINDOW_INVALID", "overlap window must be between 30 and 3600 seconds")
	}
	if config.MaxDistinctPublicIPsPerDay < 1 || config.MaxDistinctPublicIPsPerDay > 100 {
		return infraerrors.BadRequest("USER_RISK_DISTINCT_IP_LIMIT_INVALID", "max distinct public ips per day must be between 1 and 100")
	}
	if config.HighRiskActiveHoursPerDay < 1 || config.HighRiskActiveHoursPerDay > 24 {
		return infraerrors.BadRequest("USER_RISK_ACTIVE_HOURS_INVALID", "high risk active hours must be between 1 and 24")
	}
	if config.DailyScorePenaltyCap <= 0 || config.DailyScorePenaltyCap > 5 {
		return infraerrors.BadRequest("USER_RISK_DAILY_PENALTY_INVALID", "daily score penalty cap must be between 0 and 5")
	}
	if config.DailyScoreRecovery <= 0 || config.DailyScoreRecovery > 5 {
		return infraerrors.BadRequest("USER_RISK_DAILY_RECOVERY_INVALID", "daily score recovery must be between 0 and 5")
	}
	if config.RedisEvidenceRetentionDays < 1 || config.RedisEvidenceRetentionDays > 30 {
		return infraerrors.BadRequest("USER_RISK_RETENTION_DAYS_INVALID", "redis evidence retention days must be between 1 and 30")
	}
	return nil
}

type UserRiskProfile struct {
	ID                  int64                  `json:"id"`
	UserID              int64                  `json:"user_id"`
	Score               float64                `json:"score"`
	Status              string                 `json:"status"`
	ConsecutiveBadDays  int                    `json:"consecutive_bad_days"`
	LastEvaluatedAt     *time.Time             `json:"last_evaluated_at,omitempty"`
	LastWarnedAt        *time.Time             `json:"last_warned_at,omitempty"`
	GracePeriodStartedAt *time.Time            `json:"grace_period_started_at,omitempty"`
	LockedAt            *time.Time             `json:"locked_at,omitempty"`
	LockReason          string                 `json:"lock_reason,omitempty"`
	LastEvaluationSummary string               `json:"last_evaluation_summary,omitempty"`
	Exempted            bool                   `json:"exempted"`
	ExemptedAt          *time.Time             `json:"exempted_at,omitempty"`
	ExemptedBy          *int64                 `json:"exempted_by,omitempty"`
	ExemptionReason     string                 `json:"exemption_reason,omitempty"`
	UnlockedAt          *time.Time             `json:"unlocked_at,omitempty"`
	UnlockedBy          *int64                 `json:"unlocked_by,omitempty"`
	UnlockReason        string                 `json:"unlock_reason,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	SignalSnapshot      *UserRiskSignalSnapshot `json:"signal_snapshot,omitempty"`
}

func DefaultUserRiskProfile(userID int64) *UserRiskProfile {
	return &UserRiskProfile{
		UserID: userID,
		Score:  5.0,
		Status: UserRiskStatusHealthy,
	}
}

func (p *UserRiskProfile) IsLocked() bool {
	return p != nil && p.Status == UserRiskStatusLocked
}

func (p *UserRiskProfile) IsExempted() bool {
	return p != nil && p.Exempted
}

type UserRiskEvent struct {
	ID         int64             `json:"id"`
	UserID     int64             `json:"user_id"`
	EventType  string            `json:"event_type"`
	Severity   string            `json:"severity"`
	ScoreDelta float64           `json:"score_delta"`
	ScoreAfter float64           `json:"score_after"`
	Summary    string            `json:"summary"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	WindowStart *time.Time       `json:"window_start,omitempty"`
	WindowEnd   *time.Time       `json:"window_end,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

type UserRiskSignalSnapshot struct {
	DateKey                string   `json:"date_key"`
	DistinctPublicIPs      int      `json:"distinct_public_ips"`
	DistinctUserAgents     int      `json:"distinct_user_agents"`
	DistinctAPIKeys        int      `json:"distinct_api_keys"`
	OverlapEvents          int      `json:"overlap_events"`
	RecentPublicIPs        []string `json:"recent_public_ips,omitempty"`
	RecentUserAgentFamilies []string `json:"recent_user_agent_families,omitempty"`
}

type UserRiskUsageSummary struct {
	UserID           int64
	DistinctAPIKeys  int
	ActiveHours      int
	TotalActualCost  float64
	TotalRequests    int64
}

type UserRiskRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*UserRiskProfile, error)
	GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*UserRiskProfile, error)
	ListUserIDsByStatuses(ctx context.Context, statuses []string) ([]int64, error)
	UpsertProfile(ctx context.Context, profile *UserRiskProfile) (*UserRiskProfile, error)
	AppendEvent(ctx context.Context, event *UserRiskEvent) (*UserRiskEvent, error)
	ListEventsByUserID(ctx context.Context, userID int64, limit int) ([]UserRiskEvent, error)
}
