//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type tempUnschedBackoffRepoStub struct {
	mockAccountRepoForGemini
	account     *Account
	tempCalls   int
	lastUntil   time.Time
	lastUpdates map[string]any
}

func (r *tempUnschedBackoffRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.account == nil {
		return nil, ErrAccountNotFound
	}
	return r.account, nil
}

func (r *tempUnschedBackoffRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastUntil = until
	return nil
}

func (r *tempUnschedBackoffRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	r.lastUpdates = make(map[string]any, len(updates))
	for k, v := range updates {
		r.lastUpdates[k] = v
	}
	return nil
}

func TestRateLimitService_TriggerTempUnschedulable_UsesExponentialBackoffForRepeatedFailures(t *testing.T) {
	now := time.Now().UTC()
	repo := &tempUnschedBackoffRepoStub{
		account: &Account{
			ID: 1,
			Extra: map[string]any{
				"temp_unsched_runtime_consecutive_count": 1,
				"temp_unsched_runtime_last_status_code":  502,
				"temp_unsched_runtime_last_rule_id":      "rule_502",
				"temp_unsched_runtime_last_triggered_at": now.Add(-time.Minute).Format(time.RFC3339),
			},
		},
	}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID: 1,
		Credentials: map[string]any{
			"temp_unschedulable_enabled": true,
		},
	}
	rule := TempUnschedulableRule{
		ID:              "rule_502",
		ErrorCode:       502,
		Keywords:        []string{"bad gateway"},
		DurationMinutes: 1,
	}

	start := time.Now()
	ok := svc.triggerTempUnschedulable(context.Background(), account, rule, 0, 502, "bad gateway", []byte(`{"error":"bad gateway"}`))

	require.True(t, ok)
	require.Equal(t, 1, repo.tempCalls)
	require.WithinDuration(t, start.Add(2*time.Minute), repo.lastUntil, 10*time.Second)
	require.Equal(t, 2, repo.lastUpdates["temp_unsched_runtime_consecutive_count"])
	require.Equal(t, "rule_502", repo.lastUpdates["temp_unsched_runtime_last_rule_id"])
	require.Equal(t, 502, repo.lastUpdates["temp_unsched_runtime_last_status_code"])
}
