package admin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

var accountStatsCache = newSnapshotCache(60 * time.Second)

type accountStatsCacheKey struct {
	AccountID int64  `json:"account_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Include   string `json:"include"`
}

func buildAccountStatsCacheKey(accountID int64, startTime, endTime time.Time, include string) string {
	raw, err := json.Marshal(accountStatsCacheKey{
		AccountID: accountID,
		StartTime: startTime.UTC().Format(time.RFC3339),
		EndTime:   endTime.UTC().Format(time.RFC3339),
		Include:   include,
	})
	if err != nil {
		return fmt.Sprintf("account_stats:%d:%s:%s:%s", accountID, startTime.UTC().Format(time.RFC3339), endTime.UTC().Format(time.RFC3339), include)
	}
	return string(raw)
}

func accountStatsPayloadCounts(payload any) (modelsCount, endpointsCount, upstreamEndpointsCount int) {
	switch typed := payload.(type) {
	case *usagestats.AccountUsageStatsResponse:
		if typed == nil {
			return 0, 0, 0
		}
		return len(typed.Models), len(typed.Endpoints), len(typed.UpstreamEndpoints)
	case *usagestats.AccountUsageStatsPartialResponse:
		if typed == nil {
			return 0, 0, 0
		}
		return len(typed.Models), len(typed.Endpoints), len(typed.UpstreamEndpoints)
	case *usagestats.AccountUsageStatsDetailsResponse:
		if typed == nil {
			return 0, 0, 0
		}
		return len(typed.Models), len(typed.Endpoints), len(typed.UpstreamEndpoints)
	default:
		return 0, 0, 0
	}
}

func accountStatsPayloadBytes(payload any) int {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	return len(raw)
}
