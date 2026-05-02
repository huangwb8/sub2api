package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/redis/go-redis/v9" //nolint:depguard // user risk signals are ephemeral Redis-backed evidence.
)

type UserRiskSignalService struct {
	rdb            *redis.Client
	settingService *SettingService
}

func NewUserRiskSignalService(rdb *redis.Client, settingService *SettingService) *UserRiskSignalService {
	return &UserRiskSignalService{
		rdb:            rdb,
		settingService: settingService,
	}
}

func (s *UserRiskSignalService) RecordTrustedRequest(ctx context.Context, userID, apiKeyID int64, clientIP, userAgent string) error {
	if s == nil || s.rdb == nil || userID <= 0 {
		return nil
	}
	config := DefaultUserRiskControlConfig()
	if s.settingService != nil {
		if loaded, err := s.settingService.GetUserRiskControlConfig(ctx); err == nil && loaded != nil {
			config = loaded
		}
	}
	if config == nil || !config.Enabled {
		return nil
	}

	normalizedIP := ip.NormalizeRiskEvidenceIP(clientIP)
	normalizedUA := normalizeUserAgentFamily(userAgent)
	day := timezone.Today()
	dayKey := day.Format("20060102")
	retention := time.Duration(config.RedisEvidenceRetentionDays) * 24 * time.Hour
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}

	pipe := s.rdb.TxPipeline()
	if normalizedIP != "" {
		pipe.SAdd(ctx, userRiskDailyIPKey(dayKey, userID), normalizedIP)
		pipe.Expire(ctx, userRiskDailyIPKey(dayKey, userID), retention)
	}
	if normalizedUA != "" {
		pipe.SAdd(ctx, userRiskDailyUAKey(dayKey, userID), normalizedUA)
		pipe.Expire(ctx, userRiskDailyUAKey(dayKey, userID), retention)
	}
	if apiKeyID > 0 {
		pipe.SAdd(ctx, userRiskDailyAPIKeyKey(dayKey, userID), apiKeyID)
		pipe.Expire(ctx, userRiskDailyAPIKeyKey(dayKey, userID), retention)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	if normalizedIP == "" {
		return nil
	}
	return s.recordOverlapSignal(ctx, config, dayKey, userID, normalizedIP, retention)
}

func (s *UserRiskSignalService) recordOverlapSignal(ctx context.Context, config *UserRiskControlConfig, dayKey string, userID int64, normalizedIP string, retention time.Duration) error {
	now := time.Now()
	window := time.Duration(config.OverlapWindowSeconds) * time.Second
	if window <= 0 {
		window = 5 * time.Minute
	}
	activeKey := userRiskActiveIPKey(userID)
	minScore := fmt.Sprintf("-%d", now.Add(-window).Unix())

	pipe := s.rdb.TxPipeline()
	pipe.ZAdd(ctx, activeKey, redis.Z{Score: float64(now.Unix()), Member: normalizedIP})
	pipe.ZRemRangeByScore(ctx, activeKey, "-inf", minScore)
	pipe.Expire(ctx, activeKey, window+time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	activeIPs, err := s.rdb.ZRange(ctx, activeKey, 0, -1).Result()
	if err != nil {
		return err
	}
	if len(activeIPs) <= 1 {
		return nil
	}
	debounceKey := userRiskOverlapDebounceKey(userID, normalizedIP)
	created, err := s.rdb.SetNX(ctx, debounceKey, 1, window).Result()
	if err != nil || !created {
		return err
	}
	overlapKey := userRiskDailyOverlapKey(dayKey, userID)
	if err := s.rdb.Incr(ctx, overlapKey).Err(); err != nil {
		return err
	}
	return s.rdb.Expire(ctx, overlapKey, retention).Err()
}

func (s *UserRiskSignalService) GetDailySnapshot(ctx context.Context, userID int64, day time.Time) (*UserRiskSignalSnapshot, error) {
	if s == nil || s.rdb == nil || userID <= 0 {
		return &UserRiskSignalSnapshot{}, nil
	}
	dayKey := timezone.StartOfDay(day).Format("20060102")
	ipKey := userRiskDailyIPKey(dayKey, userID)
	uaKey := userRiskDailyUAKey(dayKey, userID)
	apiKeyKey := userRiskDailyAPIKeyKey(dayKey, userID)
	overlapKey := userRiskDailyOverlapKey(dayKey, userID)

	pipe := s.rdb.TxPipeline()
	ipMembers := pipe.SMembers(ctx, ipKey)
	uaMembers := pipe.SMembers(ctx, uaKey)
	apiKeyMembers := pipe.SMembers(ctx, apiKeyKey)
	overlapCount := pipe.Get(ctx, overlapKey)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	snapshot := &UserRiskSignalSnapshot{
		DateKey:                 dayKey,
		RecentPublicIPs:         ipMembers.Val(),
		RecentUserAgentFamilies: uaMembers.Val(),
		DistinctPublicIPs:       len(ipMembers.Val()),
		DistinctUserAgents:      len(uaMembers.Val()),
		DistinctAPIKeys:         len(apiKeyMembers.Val()),
	}
	if v, err := overlapCount.Int(); err == nil {
		snapshot.OverlapEvents = v
	}
	return snapshot, nil
}

func normalizeUserAgentFamily(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if idx := strings.IndexAny(raw, " /"); idx > 0 {
		raw = raw[:idx]
	}
	raw = strings.Trim(raw, "-_.")
	if raw == "" {
		return ""
	}
	return raw
}

func userRiskDailyIPKey(dayKey string, userID int64) string {
	return fmt.Sprintf("user_risk:daily:%s:user:%d:ips", dayKey, userID)
}

func userRiskDailyUAKey(dayKey string, userID int64) string {
	return fmt.Sprintf("user_risk:daily:%s:user:%d:ua", dayKey, userID)
}

func userRiskDailyAPIKeyKey(dayKey string, userID int64) string {
	return fmt.Sprintf("user_risk:daily:%s:user:%d:api_keys", dayKey, userID)
}

func userRiskDailyOverlapKey(dayKey string, userID int64) string {
	return fmt.Sprintf("user_risk:daily:%s:user:%d:overlap_count", dayKey, userID)
}

func userRiskActiveIPKey(userID int64) string {
	return fmt.Sprintf("user_risk:active:user:%d:ips", userID)
}

func userRiskOverlapDebounceKey(userID int64, normalizedIP string) string {
	return fmt.Sprintf("user_risk:overlap:user:%d:ip:%s", userID, normalizedIP)
}
