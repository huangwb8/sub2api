//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCacheSnapshotUsesSlimMetadataButKeepsFullAccount(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)

	bucket := service.SchedulerBucket{GroupID: 2, Platform: service.PlatformGemini, Mode: service.SchedulerModeSingle}
	now := time.Now().UTC().Truncate(time.Second)
	limitReset := now.Add(10 * time.Minute)
	overloadUntil := now.Add(2 * time.Minute)
	tempUnschedUntil := now.Add(3 * time.Minute)
	windowEnd := now.Add(5 * time.Hour)

	account := service.Account{
		ID:          101,
		Name:        "gemini-heavy",
		Platform:    service.PlatformGemini,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 3,
		Priority:    7,
		LastUsedAt:  &now,
		Credentials: map[string]any{
			"api_key":       "gemini-api-key",
			"access_token":  "secret-access-token",
			"project_id":    "proj-1",
			"oauth_type":    "ai_studio",
			"model_mapping": map[string]any{"gemini-2.5-pro": "gemini-2.5-pro"},
			"huge_blob":     strings.Repeat("x", 4096),
		},
		Extra: map[string]any{
			"mixed_scheduling":             true,
			"window_cost_limit":            12.5,
			"window_cost_sticky_reserve":   8.0,
			"max_sessions":                 4,
			"session_idle_timeout_minutes": 11,
			"unused_large_field":           strings.Repeat("y", 4096),
		},
		RateLimitResetAt:       &limitReset,
		OverloadUntil:          &overloadUntil,
		TempUnschedulableUntil: &tempUnschedUntil,
		SessionWindowStart:     &now,
		SessionWindowEnd:       &windowEnd,
		SessionWindowStatus:    "active",
		GroupIDs:               []int64{bucket.GroupID},
		AccountGroups: []service.AccountGroup{
			{
				AccountID: 101,
				GroupID:   bucket.GroupID,
				Priority:  5,
				Group:     &service.Group{ID: bucket.GroupID, Name: "gemini-group"},
			},
		},
	}

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))

	snapshot, hit, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, snapshot, 1)

	got := snapshot[0]
	require.NotNil(t, got)
	require.Equal(t, "gemini-api-key", got.GetCredential("api_key"))
	require.Equal(t, "proj-1", got.GetCredential("project_id"))
	require.Equal(t, "ai_studio", got.GetCredential("oauth_type"))
	require.NotEmpty(t, got.GetModelMapping())
	require.Empty(t, got.GetCredential("access_token"))
	require.Empty(t, got.GetCredential("huge_blob"))
	require.Equal(t, true, got.Extra["mixed_scheduling"])
	require.Equal(t, 12.5, got.GetWindowCostLimit())
	require.Equal(t, 8.0, got.GetWindowCostStickyReserve())
	require.Equal(t, 4, got.GetMaxSessions())
	require.Equal(t, 11, got.GetSessionIdleTimeoutMinutes())
	require.Nil(t, got.Extra["unused_large_field"])
	require.Equal(t, []int64{bucket.GroupID}, got.GroupIDs)
	require.Len(t, got.AccountGroups, 1)
	require.Equal(t, account.ID, got.AccountGroups[0].AccountID)
	require.Equal(t, bucket.GroupID, got.AccountGroups[0].GroupID)
	require.Nil(t, got.AccountGroups[0].Group)

	full, err := cache.GetAccount(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, full)
	require.Equal(t, "secret-access-token", full.GetCredential("access_token"))
	require.Equal(t, strings.Repeat("x", 4096), full.GetCredential("huge_blob"))
	require.Len(t, full.AccountGroups, 1)
	require.NotNil(t, full.AccountGroups[0].Group)
}

func TestSchedulerCache_SetSnapshot_ExpiresOldSnapshotWithGraceTTL(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)
	bucket := service.SchedulerBucket{GroupID: 3, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
	activeKey := schedulerBucketKey(schedulerActivePrefix, bucket)

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true}}))
	oldActive, err := rdb.Get(ctx, activeKey).Result()
	require.NoError(t, err)
	oldSnapshotKey := schedulerSnapshotKey(bucket, oldActive)

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{{ID: 2, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true}}))

	activeVal, err := rdb.Get(ctx, activeKey).Result()
	require.NoError(t, err)
	require.Equal(t, "2", activeVal)

	ttl, err := rdb.TTL(ctx, oldSnapshotKey).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, time.Duration(snapshotGraceTTLSeconds)*time.Second)
}

func TestSchedulerCache_ActivateSnapshotRejectsOlderVersion(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	bucket := service.SchedulerBucket{GroupID: 5, Platform: service.PlatformOpenAI, Mode: service.SchedulerModeForced}
	activeKey := schedulerBucketKey(schedulerActivePrefix, bucket)
	readyKey := schedulerBucketKey(schedulerReadyPrefix, bucket)
	oldSnapshotKey := schedulerSnapshotKey(bucket, "1")
	newerSnapshotKey := schedulerSnapshotKey(bucket, "2")

	require.NoError(t, rdb.Set(ctx, activeKey, "2", 0).Err())
	require.NoError(t, rdb.ZAdd(ctx, oldSnapshotKey, redis.Z{Score: 0, Member: "1"}).Err())
	require.NoError(t, rdb.ZAdd(ctx, newerSnapshotKey, redis.Z{Score: 0, Member: "2"}).Err())

	result, err := activateSnapshotScript.Run(ctx, rdb, []string{activeKey, readyKey, schedulerBucketSetKey, oldSnapshotKey, fmt.Sprintf("%s%d:%s:%s:v", schedulerSnapshotPrefix, bucket.GroupID, bucket.Platform, bucket.Mode)}, "1", bucket.String(), snapshotGraceTTLSeconds).Int()
	require.NoError(t, err)
	require.Equal(t, 0, result)

	activeVal, err := rdb.Get(ctx, activeKey).Result()
	require.NoError(t, err)
	require.Equal(t, "2", activeVal)

	exists, err := rdb.Exists(ctx, oldSnapshotKey).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}

func TestSchedulerCache_UnlockBucketRequiresOwnerToken(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	cache := NewSchedulerCache(rdb)
	bucket := service.SchedulerBucket{GroupID: 8, Platform: service.PlatformGemini, Mode: service.SchedulerModeMixed}
	lockKey := schedulerBucketKey(schedulerLockPrefix, bucket)

	token, ok, err := cache.TryLockBucket(ctx, bucket, 30*time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEmpty(t, token)

	require.NoError(t, cache.UnlockBucket(ctx, bucket, "wrong-token"))
	exists, err := rdb.Exists(ctx, lockKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)

	require.NoError(t, cache.UnlockBucket(ctx, bucket, token))
	exists, err = rdb.Exists(ctx, lockKey).Result()
	require.NoError(t, err)
	require.Zero(t, exists)
}
