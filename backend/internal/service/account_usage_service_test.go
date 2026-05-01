package service

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
	clearRateCh   chan int64
}

func (r *accountUsageCodexProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) ClearRateLimit(_ context.Context, id int64) error {
	if r.clearRateCh != nil {
		r.clearRateCh <- id
	}
	return nil
}

func TestShouldRefreshOpenAICodexSnapshot(t *testing.T) {
	t.Parallel()

	rateLimitedUntil := time.Now().Add(5 * time.Minute)
	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{RateLimitResetAt: &rateLimitedUntil}, usage, now) {
		t.Fatal("expected rate-limited account to force codex snapshot refresh")
	}

	if shouldRefreshOpenAICodexSnapshot(&Account{}, usage, now) {
		t.Fatal("expected complete non-rate-limited usage to skip codex snapshot refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{}, &UsageInfo{FiveHour: nil, SevenDay: &UsageProgress{}}, now) {
		t.Fatal("expected missing 5h snapshot to require refresh")
	}

	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       staleAt,
		},
	}, usage, now) {
		t.Fatal("expected stale ws snapshot to trigger refresh")
	}
}

func TestExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "100")
	headers.Set("x-codex-secondary-reset-after-seconds", "18000")
	headers.Set("x-codex-secondary-window-minutes", "300")

	updates, err := extractOpenAICodexProbeUpdates(&http.Response{StatusCode: http.StatusTooManyRequests, Header: headers})
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdates() error = %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected codex probe updates from 429 headers")
	}
	if got := updates["codex_5h_used_percent"]; got != 100.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 100", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 100.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 100", got)
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotSyncsRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:          321,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
		}}},
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}

	svc.persistOpenAICodexProbeSnapshot(321, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
	}, false)

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_7d_used_percent"]; got != 100.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 100", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		if !got.Equal(resetAt) {
			t.Fatalf("rate limit reset = %v, want %v", got, resetAt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照同步限流状态超时")
	}
}

func TestSyncOpenAICodexRateLimitFromUpdatesKeepsLongerExistingReset(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	existingReset := now.Add(4 * time.Hour)
	codexReset := now.Add(time.Hour)
	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:               654,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			RateLimitResetAt: &existingReset,
		}}},
		rateLimitCh: make(chan time.Time, 1),
	}

	got := syncOpenAICodexRateLimitFromUpdates(context.Background(), repo, 654, map[string]any{
		"codex_5h_used_percent": 100.0,
		"codex_5h_reset_at":     codexReset.Format(time.RFC3339),
	}, now)

	if got == nil || !got.Equal(existingReset) {
		t.Fatalf("reset = %v, want existing %v", got, existingReset)
	}
	select {
	case persisted := <-repo.rateLimitCh:
		t.Fatalf("不应使用较短的 Codex reset 覆盖已有上游限流: %v", persisted)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSyncOpenAICodexRateLimitFromUpdatesClearsRecoveredCodexReset(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	codexReset := now.Add(2 * time.Hour)
	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:               765,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			RateLimitResetAt: &codexReset,
			Extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
			},
		}}},
		rateLimitCh: make(chan time.Time, 1),
		clearRateCh: make(chan int64, 1),
	}

	got := syncOpenAICodexRateLimitFromUpdates(context.Background(), repo, 765, map[string]any{
		"codex_7d_used_percent": 42.0,
		"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
		"codex_5h_used_percent": 7.0,
		"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
	}, now)

	if got != nil {
		t.Fatalf("recovered codex reset = %v, want nil", got)
	}
	select {
	case id := <-repo.clearRateCh:
		if id != 765 {
			t.Fatalf("cleared account id = %d, want 765", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待恢复后的 Codex 限流清理超时")
	}
	select {
	case reset := <-repo.rateLimitCh:
		t.Fatalf("不应重新设置限流: %v", reset)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSyncOpenAICodexRateLimitFromUpdatesDoesNotClearLongerOrdinaryReset(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	codexReset := now.Add(2 * time.Hour)
	ordinaryReset := now.Add(4 * time.Hour)
	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:               876,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			RateLimitResetAt: &ordinaryReset,
			Extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
			},
		}}},
		clearRateCh: make(chan int64, 1),
	}

	got := syncOpenAICodexRateLimitFromUpdates(context.Background(), repo, 876, map[string]any{
		"codex_7d_used_percent": 42.0,
		"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
	}, now)

	if got != nil {
		t.Fatalf("reset = %v, want nil", got)
	}
	select {
	case id := <-repo.clearRateCh:
		t.Fatalf("不应清理更长的普通上游限流: account=%d", id)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSyncOpenAICodexRateLimitFromUpdatesDoesNotClearWhenClearDisabled(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	codexReset := now.Add(2 * time.Hour)
	repo := &accountUsageCodexProbeRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:               987,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			RateLimitResetAt: &codexReset,
			Extra: map[string]any{
				"codex_7d_used_percent": 100.0,
				"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
			},
		}}},
		clearRateCh: make(chan int64, 1),
	}

	_ = syncOpenAICodexRateLimitFromUpdatesWithClear(context.Background(), repo, 987, map[string]any{
		"codex_7d_used_percent": 42.0,
		"codex_7d_reset_at":     codexReset.Format(time.RFC3339),
	}, now, false)

	select {
	case id := <-repo.clearRateCh:
		t.Fatalf("不应在禁止清理时清除限流: account=%d", id)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_DoesNotPromoteCodexExtraToRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 1.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 100.0 {
		t.Fatalf("预期 7 天用量仍然可见，实际为 %#v", usage.SevenDay)
	}
	if account.RateLimitResetAt != nil {
		t.Fatalf("不应让已耗尽的 codex extra 改写运行时限流状态: %v", account.RateLimitResetAt)
	}
	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 持久化为运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	t.Run("expired 5h window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     "2026-03-16T10:00:00Z", // 2h ago
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired window, got %v", progress.Utilization)
		}
		if progress.RemainingSeconds != 0 {
			t.Fatalf("expected RemainingSeconds=0, got %v", progress.RemainingSeconds)
		}
	})

	t.Run("active 5h window keeps utilization", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Format(time.RFC3339)
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     resetAt,
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 42.0 {
			t.Fatalf("expected Utilization=42, got %v", progress.Utilization)
		}
	})

	t.Run("expired 7d window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_7d_used_percent": 88.0,
			"codex_7d_reset_at":     "2026-03-15T00:00:00Z", // yesterday
		}
		progress := buildCodexUsageProgressFromExtra(extra, "7d", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired 7d window, got %v", progress.Utilization)
		}
	})
}
