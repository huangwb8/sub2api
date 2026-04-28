package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type UserRiskService struct {
	repo                 UserRiskRepository
	userRepo             UserRepository
	usageRepo            UsageLogRepository
	settingService       *SettingService
	signalService        *UserRiskSignalService
	emailQueue           *EmailQueueService
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewUserRiskService(
	repo UserRiskRepository,
	userRepo UserRepository,
	usageRepo UsageLogRepository,
	settingService *SettingService,
	signalService *UserRiskSignalService,
	emailQueue *EmailQueueService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *UserRiskService {
	return &UserRiskService{
		repo:                 repo,
		userRepo:             userRepo,
		usageRepo:            usageRepo,
		settingService:       settingService,
		signalService:        signalService,
		emailQueue:           emailQueue,
		authCacheInvalidator: authCacheInvalidator,
	}
}

func (s *UserRiskService) GetProfile(ctx context.Context, userID int64) (*UserRiskProfile, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return DefaultUserRiskProfile(userID), nil
	}
	profile, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return DefaultUserRiskProfile(userID), nil
	}
	return profile, nil
}

func (s *UserRiskService) GetUserRiskDetails(ctx context.Context, userID int64) (*UserRiskProfile, error) {
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.signalService != nil {
		if snapshot, err := s.signalService.GetDailySnapshot(ctx, userID, timezone.Now()); err == nil {
			profile.SignalSnapshot = snapshot
		}
	}
	return profile, nil
}

func (s *UserRiskService) ListEventsByUserID(ctx context.Context, userID int64, limit int) ([]UserRiskEvent, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListEventsByUserID(ctx, userID, limit)
}

func (s *UserRiskService) CheckAccess(ctx context.Context, userID int64) (*UserRiskProfile, error) {
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if profile != nil && profile.IsLocked() {
		message := ErrUserRiskLocked.Message
		if s.settingService != nil {
			if config, cfgErr := s.settingService.GetUserRiskControlConfig(ctx); cfgErr == nil && config != nil && strings.TrimSpace(config.LockMessage) != "" {
				message = strings.TrimSpace(config.LockMessage)
			}
		}
		return profile, infraerrors.Forbidden("USER_RISK_LOCKED", message)
	}
	return profile, nil
}

func (s *UserRiskService) RunDailyEvaluation(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil || s.usageRepo == nil || s.settingService == nil {
		return nil
	}
	config, err := s.settingService.GetUserRiskControlConfig(ctx)
	if err != nil || config == nil || !config.Enabled {
		return err
	}

	targetDay := timezone.StartOfDay(now).Add(-24 * time.Hour)
	windowStart := timezone.StartOfDay(targetDay)
	windowEnd := windowStart.Add(24 * time.Hour)

	usageUsers, err := s.usageRepo.ListUserIDsWithUsageBetween(ctx, windowStart, windowEnd)
	if err != nil {
		return err
	}
	reviewUsers, err := s.repo.ListUserIDsByStatuses(ctx, []string{
		UserRiskStatusObserved,
		UserRiskStatusWarned,
		UserRiskStatusGracePeriod,
		UserRiskStatusExempted,
	})
	if err != nil {
		return err
	}
	userIDSet := make(map[int64]struct{}, len(usageUsers)+len(reviewUsers))
	for _, userID := range usageUsers {
		userIDSet[userID] = struct{}{}
	}
	for _, userID := range reviewUsers {
		userIDSet[userID] = struct{}{}
	}
	userIDs := make([]int64, 0, len(userIDSet))
	for userID := range userIDSet {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	for _, userID := range userIDs {
		if err := s.evaluateUserDay(ctx, userID, windowStart, windowEnd, config); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserRiskService) evaluateUserDay(ctx context.Context, userID int64, windowStart, windowEnd time.Time, config *UserRiskControlConfig) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil
	}
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return err
	}
	if profile.IsLocked() {
		return nil
	}

	snapshot := &UserRiskSignalSnapshot{}
	if s.signalService != nil {
		snapshot, err = s.signalService.GetDailySnapshot(ctx, userID, windowStart)
		if err != nil {
			return err
		}
	}
	usageSummary, err := s.usageRepo.GetUserRiskUsageSummary(ctx, userID, windowStart, windowEnd)
	if err != nil {
		return err
	}
	if usageSummary == nil {
		usageSummary = &UserRiskUsageSummary{UserID: userID}
	}

	profile.SignalSnapshot = snapshot
	prevScore := profile.Score
	if prevScore <= 0 {
		prevScore = 5.0
	}
	prevStatus := profile.Status
	if prevStatus == "" {
		prevStatus = UserRiskStatusHealthy
	}

	now := time.Now()
	reasons, penalty := scoreUserRiskDay(config, snapshot, usageSummary)
	if profile.IsExempted() {
		profile.Status = UserRiskStatusExempted
		profile.LastEvaluatedAt = &now
		profile.LastEvaluationSummary = buildUserRiskSummary(reasons, usageSummary, snapshot, 0, prevScore)
		_, err = s.repo.UpsertProfile(ctx, profile)
		return err
	}

	if penalty > 0 {
		profile.Score = clampUserRiskScore(prevScore - penalty)
		profile.ConsecutiveBadDays++
	} else {
		profile.Score = clampUserRiskScore(prevScore + config.DailyScoreRecovery)
		profile.ConsecutiveBadDays = 0
	}

	if profile.Score < config.WarningThreshold {
		switch config.Mode {
		case UserRiskControlModeObserveOnly:
			profile.Status = UserRiskStatusObserved
		default:
			if profile.ConsecutiveBadDays <= 1 {
				profile.Status = UserRiskStatusWarned
			} else {
				profile.Status = UserRiskStatusGracePeriod
			}
		}
	} else {
		profile.Status = UserRiskStatusHealthy
		profile.GracePeriodStartedAt = nil
	}

	if profile.Status == UserRiskStatusWarned {
		profile.LastWarnedAt = &now
		if profile.GracePeriodStartedAt == nil {
			profile.GracePeriodStartedAt = &now
		}
		if err := s.sendWarningEmail(ctx, user, profile, config, reasons); err != nil {
			return err
		}
	}
	if profile.Status == UserRiskStatusGracePeriod && profile.GracePeriodStartedAt == nil {
		profile.GracePeriodStartedAt = &now
	}

	if shouldAutoLockUserRiskProfile(config, profile, snapshot) {
		profile.Status = UserRiskStatusLocked
		profile.LockedAt = &now
		profile.LockReason = firstNonEmpty(strings.Join(reasons, "; "), "Repeated suspicious multi-IP or multi-key usage")
	}

	profile.LastEvaluatedAt = &now
	profile.LastEvaluationSummary = buildUserRiskSummary(reasons, usageSummary, snapshot, penalty, prevScore)

	updated, err := s.repo.UpsertProfile(ctx, profile)
	if err != nil {
		return err
	}
	if updated != nil {
		profile = updated
	}
	if s.authCacheInvalidator != nil && (profile.Status == UserRiskStatusLocked || prevStatus == UserRiskStatusLocked || prevStatus != profile.Status) {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}

	if penalty > 0 || prevStatus != profile.Status || prevScore != profile.Score {
		severity := UserRiskSeverityInfo
		eventType := UserRiskEventTypeDailyEvaluation
		if profile.Status == UserRiskStatusLocked {
			severity = UserRiskSeverityCritical
			eventType = UserRiskEventTypeLocked
		} else if penalty > 0 {
			severity = UserRiskSeverityWarning
		}
		if _, err := s.repo.AppendEvent(ctx, &UserRiskEvent{
			UserID:      userID,
			EventType:   eventType,
			Severity:    severity,
			ScoreDelta:  profile.Score - prevScore,
			ScoreAfter:  profile.Score,
			Summary:     profile.LastEvaluationSummary,
			WindowStart: &windowStart,
			WindowEnd:   &windowEnd,
			Metadata: map[string]any{
				"reasons":              reasons,
				"previous_status":      prevStatus,
				"current_status":       profile.Status,
				"consecutive_bad_days": profile.ConsecutiveBadDays,
			},
		}); err != nil {
			return err
		}
	}
	if profile.Status == UserRiskStatusWarned && prevStatus != UserRiskStatusWarned {
		if _, err := s.repo.AppendEvent(ctx, &UserRiskEvent{
			UserID:      userID,
			EventType:   UserRiskEventTypeWarned,
			Severity:    UserRiskSeverityWarning,
			ScoreDelta:  profile.Score - prevScore,
			ScoreAfter:  profile.Score,
			Summary:     "Risk warning email sent and grace period started",
			WindowStart: &windowStart,
			WindowEnd:   &windowEnd,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserRiskService) UnlockUser(ctx context.Context, userID, actorID int64, reason string) (*UserRiskProfile, error) {
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	profile.Score = maxUserRiskFloat(profile.Score, DefaultUserRiskControlConfig().WarningThreshold)
	profile.Status = UserRiskStatusHealthy
	profile.ConsecutiveBadDays = 0
	profile.LockedAt = nil
	profile.LockReason = ""
	profile.GracePeriodStartedAt = nil
	profile.UnlockedAt = &now
	profile.UnlockedBy = &actorID
	profile.UnlockReason = strings.TrimSpace(reason)
	profile.LastEvaluationSummary = "Risk lock cleared manually by administrator"

	updated, err := s.repo.UpsertProfile(ctx, profile)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.AppendEvent(ctx, &UserRiskEvent{
		UserID:     userID,
		EventType:  UserRiskEventTypeUnlocked,
		Severity:   UserRiskSeverityInfo,
		ScoreDelta: 0,
		ScoreAfter: updated.Score,
		Summary:    firstNonEmpty(strings.TrimSpace(reason), "Risk lock cleared manually"),
		Metadata: map[string]any{
			"actor_id": actorID,
		},
	}); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return updated, nil
}

func (s *UserRiskService) SetExemption(ctx context.Context, userID, actorID int64, exempted bool, reason string) (*UserRiskProfile, error) {
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	profile.Exempted = exempted
	profile.ExemptionReason = strings.TrimSpace(reason)
	if exempted {
		profile.Status = UserRiskStatusExempted
		profile.ExemptedAt = &now
		profile.ExemptedBy = &actorID
	} else {
		profile.Status = UserRiskStatusHealthy
		profile.ExemptedAt = nil
		profile.ExemptedBy = nil
		profile.ConsecutiveBadDays = 0
	}
	updated, err := s.repo.UpsertProfile(ctx, profile)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.AppendEvent(ctx, &UserRiskEvent{
		UserID:     userID,
		EventType:  UserRiskEventTypeExemption,
		Severity:   UserRiskSeverityInfo,
		ScoreAfter: updated.Score,
		Summary:    firstNonEmpty(strings.TrimSpace(reason), "Risk exemption changed"),
		Metadata: map[string]any{
			"actor_id": actorID,
			"exempted": exempted,
		},
	}); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return updated, nil
}

func (s *UserRiskService) ResetScore(ctx context.Context, userID, actorID int64, reason string) (*UserRiskProfile, error) {
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.Score = 5.0
	profile.Status = UserRiskStatusHealthy
	profile.ConsecutiveBadDays = 0
	profile.LockedAt = nil
	profile.LockReason = ""
	profile.GracePeriodStartedAt = nil
	profile.LastEvaluationSummary = "Risk score reset manually by administrator"
	updated, err := s.repo.UpsertProfile(ctx, profile)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.AppendEvent(ctx, &UserRiskEvent{
		UserID:     userID,
		EventType:  UserRiskEventTypeScoreReset,
		Severity:   UserRiskSeverityInfo,
		ScoreAfter: updated.Score,
		Summary:    firstNonEmpty(strings.TrimSpace(reason), "Risk score reset manually"),
		Metadata: map[string]any{
			"actor_id": actorID,
		},
	}); err != nil {
		return nil, err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return updated, nil
}

func (s *UserRiskService) SendWarning(ctx context.Context, userID int64) (*UserRiskProfile, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile, err := s.GetUserRiskDetails(ctx, userID)
	if err != nil {
		return nil, err
	}
	config, err := s.settingService.GetUserRiskControlConfig(ctx)
	if err != nil {
		return nil, err
	}
	reasons := []string{"Administrator triggered a manual warning for suspicious usage."}
	if profile.SignalSnapshot != nil {
		reasons = append(reasons, formatRiskSignalReasons(profile.SignalSnapshot, &UserRiskUsageSummary{})...)
	}
	now := time.Now()
	profile.Status = UserRiskStatusWarned
	profile.LastWarnedAt = &now
	if profile.GracePeriodStartedAt == nil {
		profile.GracePeriodStartedAt = &now
	}
	updated, err := s.repo.UpsertProfile(ctx, profile)
	if err != nil {
		return nil, err
	}
	if err := s.sendWarningEmail(ctx, user, updated, config, reasons); err != nil {
		return nil, err
	}
	if _, err := s.repo.AppendEvent(ctx, &UserRiskEvent{
		UserID:     userID,
		EventType:  UserRiskEventTypeWarned,
		Severity:   UserRiskSeverityWarning,
		ScoreAfter: updated.Score,
		Summary:    "Manual risk warning email sent by administrator",
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

func scoreUserRiskDay(config *UserRiskControlConfig, snapshot *UserRiskSignalSnapshot, usageSummary *UserRiskUsageSummary) ([]string, float64) {
	if config == nil {
		config = DefaultUserRiskControlConfig()
	}
	if snapshot == nil {
		snapshot = &UserRiskSignalSnapshot{}
	}
	if usageSummary == nil {
		usageSummary = &UserRiskUsageSummary{}
	}
	reasons := formatRiskSignalReasons(snapshot, usageSummary)
	penalty := 0.0
	if snapshot.OverlapEvents > 0 {
		penalty += 0.75
	}
	if snapshot.DistinctPublicIPs > config.MaxDistinctPublicIPsPerDay {
		penalty += minFloat(0.5, 0.15*float64(snapshot.DistinctPublicIPs-config.MaxDistinctPublicIPsPerDay))
	}
	if snapshot.DistinctUserAgents >= 4 {
		penalty += 0.15
	}
	if usageSummary.DistinctAPIKeys >= 2 && snapshot.DistinctPublicIPs >= 2 {
		penalty += 0.25
	}
	if usageSummary.ActiveHours >= config.HighRiskActiveHoursPerDay {
		penalty += 0.2
	}
	return reasons, minFloat(config.DailyScorePenaltyCap, penalty)
}

func formatRiskSignalReasons(snapshot *UserRiskSignalSnapshot, usageSummary *UserRiskUsageSummary) []string {
	reasons := make([]string, 0, 5)
	if snapshot != nil {
		if snapshot.OverlapEvents > 0 {
			reasons = append(reasons, fmt.Sprintf("Detected %d multi-IP overlap events within the evaluation window", snapshot.OverlapEvents))
		}
		if snapshot.DistinctPublicIPs > 1 {
			reasons = append(reasons, fmt.Sprintf("Observed %d distinct trusted public IPs in one day", snapshot.DistinctPublicIPs))
		}
		if snapshot.DistinctUserAgents > 1 {
			reasons = append(reasons, fmt.Sprintf("Observed %d distinct user-agent families in one day", snapshot.DistinctUserAgents))
		}
	}
	if usageSummary != nil {
		if usageSummary.DistinctAPIKeys > 1 {
			reasons = append(reasons, fmt.Sprintf("Used %d different API keys in the same evaluation window", usageSummary.DistinctAPIKeys))
		}
		if usageSummary.ActiveHours > 0 {
			reasons = append(reasons, fmt.Sprintf("Account was active in %d hourly buckets", usageSummary.ActiveHours))
		}
	}
	return reasons
}

func shouldAutoLockUserRiskProfile(config *UserRiskControlConfig, profile *UserRiskProfile, snapshot *UserRiskSignalSnapshot) bool {
	if config == nil || profile == nil {
		return false
	}
	if config.Mode != UserRiskControlModeAutoLock {
		return false
	}
	if profile.Score >= config.LockThreshold || profile.ConsecutiveBadDays < config.AutoLockAfterConsecutiveBadDays {
		return false
	}
	if !config.RequireTrustedProxyForAutoLock {
		return true
	}
	return snapshot != nil && (snapshot.OverlapEvents > 0 || snapshot.DistinctPublicIPs > 0)
}

func buildUserRiskSummary(reasons []string, usageSummary *UserRiskUsageSummary, snapshot *UserRiskSignalSnapshot, penalty, previousScore float64) string {
	parts := make([]string, 0, len(reasons)+2)
	if len(reasons) == 0 {
		parts = append(parts, "No suspicious signals detected; applied recovery path.")
	} else {
		parts = append(parts, reasons...)
	}
	parts = append(parts, fmt.Sprintf("score delta %.2f from previous %.2f", -penalty, previousScore))
	if usageSummary != nil && usageSummary.TotalRequests > 0 {
		parts = append(parts, fmt.Sprintf("requests=%d active_hours=%d", usageSummary.TotalRequests, usageSummary.ActiveHours))
	}
	if snapshot != nil && snapshot.DistinctPublicIPs > 0 {
		parts = append(parts, fmt.Sprintf("trusted_public_ips=%d", snapshot.DistinctPublicIPs))
	}
	return strings.Join(parts, "; ")
}

func clampUserRiskScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 5 {
		return 5
	}
	return score
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxUserRiskFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
