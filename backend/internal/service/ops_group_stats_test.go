//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type opsStatsAccountRepoStub struct {
	accountRepoStub
	accounts []Account
}

func (s *opsStatsAccountRepoStub) ListWithFilters(_ context.Context, params pagination.PaginationParams, platform, _, _, _ string, _ int64, _ string) ([]Account, *pagination.PaginationResult, error) {
	filtered := make([]Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		if platform != "" && acc.Platform != platform {
			continue
		}
		filtered = append(filtered, acc)
	}
	return filtered, &pagination.PaginationResult{
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    int64(len(filtered)),
	}, nil
}

func TestOpsGroupAvailabilityUsesPrimaryGroupForUnfilteredAggregation(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	resetAt := now.Add(time.Hour)
	repo := &opsStatsAccountRepoStub{accounts: opsGroupStatsAccounts(resetAt)}
	svc := NewOpsService(nil, nil, nil, repo, nil, nil, nil, nil, nil, nil, nil)

	platformStats, groupStats, accountStats, _, err := svc.GetAccountAvailabilityStats(context.Background(), PlatformOpenAI, nil)
	require.NoError(t, err)

	require.Equal(t, int64(3), platformStats[PlatformOpenAI].TotalAccounts)
	require.Equal(t, int64(2), groupStats[1].TotalAccounts)
	require.Equal(t, int64(2), groupStats[1].AvailableCount)
	require.Equal(t, int64(1), groupStats[2].TotalAccounts)
	require.Equal(t, int64(1), groupStats[2].RateLimitCount)
	require.Equal(t, int64(1), accountStats[1].GroupID)
}

func TestOpsGroupAvailabilityUsesFilteredGroupForFilteredAggregation(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().UTC().Add(time.Hour)
	groupID := int64(2)
	repo := &opsStatsAccountRepoStub{accounts: opsGroupStatsAccounts(resetAt)}
	svc := NewOpsService(nil, nil, nil, repo, nil, nil, nil, nil, nil, nil, nil)

	_, groupStats, accountStats, _, err := svc.GetAccountAvailabilityStats(context.Background(), PlatformOpenAI, &groupID)
	require.NoError(t, err)

	require.Len(t, accountStats, 2)
	require.Equal(t, int64(2), groupStats[2].TotalAccounts)
	require.Equal(t, int64(1), groupStats[2].AvailableCount)
	require.Equal(t, int64(1), groupStats[2].RateLimitCount)
	require.Equal(t, int64(2), accountStats[1].GroupID)
}

func TestOpsAccountAvailabilityTreatsOpenAICodexExhaustedAsRateLimited(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().UTC().Add(time.Hour)
	repo := &opsStatsAccountRepoStub{accounts: []Account{
		{
			ID:          11,
			Name:        "codex-exhausted",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Extra: map[string]any{
				"codex_5h_used_percent": 100.0,
				"codex_5h_reset_at":     resetAt.Format(time.RFC3339),
			},
			Groups: []*Group{{ID: 1, Name: "GPT_Standard", Platform: PlatformOpenAI}},
		},
	}}
	svc := NewOpsService(nil, nil, nil, repo, nil, nil, nil, nil, nil, nil, nil)

	platformStats, groupStats, accountStats, _, err := svc.GetAccountAvailabilityStats(context.Background(), PlatformOpenAI, nil)
	require.NoError(t, err)

	require.Equal(t, int64(1), platformStats[PlatformOpenAI].TotalAccounts)
	require.Equal(t, int64(0), platformStats[PlatformOpenAI].AvailableCount)
	require.Equal(t, int64(1), platformStats[PlatformOpenAI].RateLimitCount)
	require.Equal(t, int64(0), groupStats[1].AvailableCount)
	require.True(t, accountStats[11].IsRateLimited)
	require.False(t, accountStats[11].IsAvailable)
	require.NotNil(t, accountStats[11].RateLimitResetAt)
}

func TestOpsGroupConcurrencyUsesPrimaryGroupForUnfilteredAggregation(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().UTC().Add(time.Hour)
	repo := &opsStatsAccountRepoStub{accounts: opsGroupStatsAccounts(resetAt)}
	cache := &stubConcurrencyCacheForTest{loadBatch: opsGroupStatsLoadBatch()}
	svc := NewOpsService(nil, nil, nil, repo, nil, NewConcurrencyService(cache), nil, nil, nil, nil, nil)

	platformStats, groupStats, accountStats, _, err := svc.GetConcurrencyStats(context.Background(), PlatformOpenAI, nil)
	require.NoError(t, err)

	require.Equal(t, int64(19), platformStats[PlatformOpenAI].MaxCapacity)
	require.Equal(t, int64(14), groupStats[1].MaxCapacity)
	require.Equal(t, int64(4), groupStats[1].CurrentInUse)
	require.Equal(t, int64(5), groupStats[2].MaxCapacity)
	require.Equal(t, int64(2), groupStats[2].CurrentInUse)
	require.Equal(t, int64(1), accountStats[1].GroupID)
}

func TestOpsGroupConcurrencyUsesFilteredGroupForFilteredAggregation(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().UTC().Add(time.Hour)
	groupID := int64(2)
	repo := &opsStatsAccountRepoStub{accounts: opsGroupStatsAccounts(resetAt)}
	cache := &stubConcurrencyCacheForTest{loadBatch: opsGroupStatsLoadBatch()}
	svc := NewOpsService(nil, nil, nil, repo, nil, NewConcurrencyService(cache), nil, nil, nil, nil, nil)

	_, groupStats, accountStats, _, err := svc.GetConcurrencyStats(context.Background(), PlatformOpenAI, &groupID)
	require.NoError(t, err)

	require.Len(t, accountStats, 2)
	require.Equal(t, int64(15), groupStats[2].MaxCapacity)
	require.Equal(t, int64(5), groupStats[2].CurrentInUse)
	require.Equal(t, int64(3), groupStats[2].WaitingInQueue)
	require.Equal(t, int64(2), accountStats[1].GroupID)
}

func opsGroupStatsAccounts(resetAt time.Time) []Account {
	standard := &Group{ID: 1, Name: "GPT_Standard", Platform: PlatformOpenAI}
	premium := &Group{ID: 2, Name: "GPT_Premium", Platform: PlatformOpenAI}
	return []Account{
		{
			ID:          1,
			Name:        "multi-group",
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 10,
			Groups:      []*Group{standard, premium},
		},
		{
			ID:               2,
			Name:             "premium-limited",
			Platform:         PlatformOpenAI,
			Status:           StatusActive,
			Schedulable:      true,
			Concurrency:      5,
			RateLimitResetAt: &resetAt,
			Groups:           []*Group{premium},
		},
		{
			ID:          3,
			Name:        "standard-only",
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 4,
			Groups:      []*Group{standard},
		},
	}
}

func opsGroupStatsLoadBatch() map[int64]*AccountLoadInfo {
	return map[int64]*AccountLoadInfo{
		1: {AccountID: 1, CurrentConcurrency: 3, WaitingCount: 1},
		2: {AccountID: 2, CurrentConcurrency: 2, WaitingCount: 2},
		3: {AccountID: 3, CurrentConcurrency: 1, WaitingCount: 0},
	}
}
