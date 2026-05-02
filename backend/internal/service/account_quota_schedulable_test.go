//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountIsSchedulable_QuotaExceededForAPIKeyOrBedrock(t *testing.T) {
	now := time.Now()
	future := now.Add(30 * time.Minute)

	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "apikey total quota exceeded is not schedulable",
			account: &Account{
				Status:      StatusActive,
				Type:        AccountTypeAPIKey,
				Schedulable: true,
				Extra: map[string]any{
					"quota_limit": 10.0,
					"quota_used":  10.0,
				},
			},
			want: false,
		},
		{
			name: "chatapi total quota exceeded is not schedulable",
			account: &Account{
				Status:      StatusActive,
				Type:        AccountTypeChatAPI,
				Schedulable: true,
				Extra: map[string]any{
					"quota_limit": 10.0,
					"quota_used":  10.0,
				},
			},
			want: false,
		},
		{
			name: "bedrock daily quota exceeded is not schedulable",
			account: &Account{
				Status:      StatusActive,
				Type:        AccountTypeBedrock,
				Schedulable: true,
				Extra: map[string]any{
					"quota_daily_limit": 10.0,
					"quota_daily_used":  10.0,
					"quota_daily_start": now.Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "apikey fixed weekly quota already reset stays schedulable",
			account: &Account{
				Status:      StatusActive,
				Type:        AccountTypeAPIKey,
				Schedulable: true,
				Extra: map[string]any{
					"quota_weekly_limit":      10.0,
					"quota_weekly_used":       10.0,
					"quota_weekly_start":      now.Add(-8 * 24 * time.Hour).Format(time.RFC3339),
					"quota_weekly_reset_mode": "fixed",
				},
			},
			want: true,
		},
		{
			name: "oauth quota fields do not affect schedulable",
			account: &Account{
				Status:      StatusActive,
				Type:        AccountTypeOAuth,
				Schedulable: true,
				Extra: map[string]any{
					"quota_limit": 10.0,
					"quota_used":  10.0,
				},
			},
			want: true,
		},
		{
			name: "openai codex 5h exhausted from snapshot is not schedulable",
			account: &Account{
				Status:      StatusActive,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Schedulable: true,
				Extra: map[string]any{
					"codex_5h_used_percent": 100.0,
					"codex_5h_reset_at":     future.UTC().Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "openai codex 7d exhausted from snapshot is not schedulable",
			account: &Account{
				Status:      StatusActive,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Schedulable: true,
				Extra: map[string]any{
					"codex_7d_used_percent": 100.0,
					"codex_7d_reset_at":     future.UTC().Format(time.RFC3339),
				},
			},
			want: false,
		},
		{
			name: "non-openai account ignores codex exhausted snapshot",
			account: &Account{
				Status:      StatusActive,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Schedulable: true,
				Extra: map[string]any{
					"codex_7d_used_percent": 100.0,
					"codex_7d_reset_at":     future.UTC().Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "openai codex exhausted snapshot after reset stays schedulable",
			account: &Account{
				Status:      StatusActive,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Schedulable: true,
				Extra: map[string]any{
					"codex_7d_used_percent": 100.0,
					"codex_7d_reset_at":     now.Add(-time.Minute).UTC().Format(time.RFC3339),
				},
			},
			want: true,
		},
		{
			name: "other blocking conditions still apply",
			account: &Account{
				Status:                 StatusActive,
				Type:                   AccountTypeAPIKey,
				Schedulable:            true,
				OverloadUntil:          &future,
				RateLimitResetAt:       nil,
				TempUnschedulableUntil: nil,
				Extra: map[string]any{
					"quota_limit": 100.0,
					"quota_used":  1.0,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsSchedulable())
		})
	}
}
