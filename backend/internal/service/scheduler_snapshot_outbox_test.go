//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type schedulerSnapshotOutboxCache struct {
	SchedulerCache
	lockBuckets        []SchedulerBucket
	unlockBuckets      []SchedulerBucket
	lockTokens         []string
	unlockTokens       []string
	setWatermarkCalls  []context.Context
	watermark          int64
	failWatermarkTimes int
}

func (c *schedulerSnapshotOutboxCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}

func (c *schedulerSnapshotOutboxCache) SetSnapshot(context.Context, SchedulerBucket, []Account) error {
	return nil
}

func (c *schedulerSnapshotOutboxCache) GetAccount(context.Context, int64) (*Account, error) {
	return nil, nil
}

func (c *schedulerSnapshotOutboxCache) SetAccount(context.Context, *Account) error {
	return nil
}

func (c *schedulerSnapshotOutboxCache) DeleteAccount(context.Context, int64) error {
	return nil
}

func (c *schedulerSnapshotOutboxCache) UpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}

func (c *schedulerSnapshotOutboxCache) TryLockBucket(_ context.Context, bucket SchedulerBucket, _ time.Duration) (string, bool, error) {
	c.lockBuckets = append(c.lockBuckets, bucket)
	token := bucket.String()
	c.lockTokens = append(c.lockTokens, token)
	return token, true, nil
}

func (c *schedulerSnapshotOutboxCache) UnlockBucket(_ context.Context, bucket SchedulerBucket, token string) error {
	c.unlockBuckets = append(c.unlockBuckets, bucket)
	c.unlockTokens = append(c.unlockTokens, token)
	return nil
}

func (c *schedulerSnapshotOutboxCache) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *schedulerSnapshotOutboxCache) GetOutboxWatermark(context.Context) (int64, error) {
	return c.watermark, nil
}

func (c *schedulerSnapshotOutboxCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	c.setWatermarkCalls = append(c.setWatermarkCalls, ctx)
	if len(c.setWatermarkCalls) <= c.failWatermarkTimes {
		return errors.New("transient watermark failure")
	}
	c.watermark = id
	return nil
}

type schedulerSnapshotOutboxRepo struct {
	events []SchedulerOutboxEvent
}

func (r *schedulerSnapshotOutboxRepo) ListAfter(context.Context, int64, int) ([]SchedulerOutboxEvent, error) {
	return r.events, nil
}

func (r *schedulerSnapshotOutboxRepo) MaxID(context.Context) (int64, error) {
	if len(r.events) == 0 {
		return 0, nil
	}
	return r.events[len(r.events)-1].ID, nil
}

func TestSchedulerSnapshotService_PollOutboxRetriesWatermarkWithFreshContext(t *testing.T) {
	cache := &schedulerSnapshotOutboxCache{
		watermark:          41,
		failWatermarkTimes: 2,
	}
	repo := &schedulerSnapshotOutboxRepo{
		events: []SchedulerOutboxEvent{
			{
				ID:        42,
				EventType: SchedulerOutboxEventAccountLastUsed,
				Payload: map[string]any{
					"last_used": map[string]any{"1": time.Now().Unix()},
				},
				CreatedAt: time.Now(),
			},
		},
	}

	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)
	svc.pollOutbox()

	require.Equal(t, int64(42), cache.watermark)
	require.Len(t, cache.setWatermarkCalls, 3)
	require.NotSame(t, cache.setWatermarkCalls[0], cache.setWatermarkCalls[1])
	require.NotSame(t, cache.setWatermarkCalls[1], cache.setWatermarkCalls[2])
}

func TestSchedulerSnapshotService_PollOutboxDeduplicatesGroupRebuildsWithinBatch(t *testing.T) {
	groupID := int64(77)
	accountID := int64(101)
	account := Account{
		ID:          accountID,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		GroupIDs:    []int64{groupID},
	}

	cache := &schedulerSnapshotOutboxCache{}
	repo := &schedulerSnapshotOutboxRepo{
		events: []SchedulerOutboxEvent{
			{
				ID:        1,
				EventType: SchedulerOutboxEventAccountChanged,
				AccountID: &accountID,
				Payload:   map[string]any{"group_ids": []any{groupID}},
				CreatedAt: time.Now(),
			},
			{
				ID:        2,
				EventType: SchedulerOutboxEventAccountChanged,
				AccountID: &accountID,
				Payload:   map[string]any{"group_ids": []any{groupID}},
				CreatedAt: time.Now(),
			},
		},
	}

	svc := NewSchedulerSnapshotService(
		cache,
		repo,
		stubOpenAIAccountRepo{accounts: []Account{account}},
		nil,
		nil,
	)

	svc.pollOutbox()

	require.Len(t, cache.lockBuckets, 2, "同一批次内相同 group/platform 只应重建一次 single+forced bucket")
	require.Equal(t, SchedulerBucket{GroupID: groupID, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}, cache.lockBuckets[0])
	require.Equal(t, SchedulerBucket{GroupID: groupID, Platform: PlatformOpenAI, Mode: SchedulerModeForced}, cache.lockBuckets[1])
	require.Equal(t, cache.lockBuckets, cache.unlockBuckets)
	require.Equal(t, cache.lockTokens, cache.unlockTokens)
}

func TestSchedulerSnapshotService_RebuildBucketUnlocksAfterFailure(t *testing.T) {
	cache := &schedulerSnapshotOutboxCache{}
	svc := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)
	bucket := SchedulerBucket{GroupID: 3, Platform: PlatformAnthropic, Mode: SchedulerModeSingle}

	err := svc.rebuildBucket(context.Background(), bucket, "unit_test_failure")
	require.Error(t, err)
	require.Equal(t, []SchedulerBucket{bucket}, cache.lockBuckets)
	require.Equal(t, []SchedulerBucket{bucket}, cache.unlockBuckets)
	require.Equal(t, cache.lockTokens, cache.unlockTokens)
}
