package service

import (
	"context"
	"errors"
	"time"
)

// GetAccountAvailabilityStats returns current account availability stats.
//
// Query-level filtering is intentionally limited to platform/group to match the dashboard scope.
func (s *OpsService) GetAccountAvailabilityStats(ctx context.Context, platformFilter string, groupIDFilter *int64) (
	map[string]*PlatformAvailability,
	map[int64]*GroupAvailability,
	map[int64]*AccountAvailability,
	*time.Time,
	error,
) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, nil, nil, nil, err
	}

	accounts, err := s.listAllAccountsForOps(ctx, platformFilter)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if groupIDFilter != nil && *groupIDFilter > 0 {
		filtered := make([]Account, 0, len(accounts))
		for _, acc := range accounts {
			for _, grp := range acc.Groups {
				if grp != nil && grp.ID == *groupIDFilter {
					filtered = append(filtered, acc)
					break
				}
			}
		}
		accounts = filtered
	}

	now := time.Now()
	collectedAt := now

	platform := make(map[string]*PlatformAvailability)
	group := make(map[int64]*GroupAvailability)
	account := make(map[int64]*AccountAvailability)

	for _, acc := range accounts {
		if acc.ID <= 0 {
			continue
		}

		availability := acc.EffectiveAvailability(now)
		isAvailable := availability.IsAvailable
		isRateLimited := availability.Status == AccountEffectiveStatusRateLimited
		isOverloaded := availability.Status == AccountEffectiveStatusOverloaded
		isTempUnsched := availability.Status == AccountEffectiveStatusTempUnschedulable
		hasError := availability.Status == AccountEffectiveStatusError

		if acc.Platform != "" {
			if _, ok := platform[acc.Platform]; !ok {
				platform[acc.Platform] = &PlatformAvailability{
					Platform: acc.Platform,
				}
			}
			p := platform[acc.Platform]
			p.TotalAccounts++
			if isAvailable {
				p.AvailableCount++
			}
			if isRateLimited {
				p.RateLimitCount++
			}
			if hasError {
				p.ErrorCount++
			}
		}

		if grp := selectOpsAggregationGroup(acc, groupIDFilter); grp != nil {
			if _, ok := group[grp.ID]; !ok {
				group[grp.ID] = &GroupAvailability{
					GroupID:   grp.ID,
					GroupName: grp.Name,
					Platform:  grp.Platform,
				}
			}
			g := group[grp.ID]
			g.TotalAccounts++
			if isAvailable {
				g.AvailableCount++
			}
			if isRateLimited {
				g.RateLimitCount++
			}
			if hasError {
				g.ErrorCount++
			}
		}

		displayGroupID := int64(0)
		displayGroupName := ""
		if grp := selectOpsAggregationGroup(acc, groupIDFilter); grp != nil {
			displayGroupID = grp.ID
			displayGroupName = grp.Name
		}

		item := &AccountAvailability{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			Platform:    acc.Platform,
			GroupID:     displayGroupID,
			GroupName:   displayGroupName,
			Status:      acc.Status,

			IsAvailable:   isAvailable,
			IsRateLimited: isRateLimited,
			IsOverloaded:  isOverloaded,
			HasError:      hasError,

			ErrorMessage: acc.ErrorMessage,
		}

		if isRateLimited && availability.EffectiveRateLimitAt != nil {
			item.RateLimitResetAt = availability.EffectiveRateLimitAt
			remainingSec := int64(availability.EffectiveRateLimitAt.Sub(now).Seconds())
			if remainingSec > 0 {
				item.RateLimitRemainingSec = &remainingSec
			}
		}
		if isOverloaded && availability.OverloadUntil != nil {
			item.OverloadUntil = availability.OverloadUntil
			remainingSec := int64(availability.OverloadUntil.Sub(now).Seconds())
			if remainingSec > 0 {
				item.OverloadRemainingSec = &remainingSec
			}
		}
		if isTempUnsched && availability.TempUnschedulableUntil != nil {
			item.TempUnschedulableUntil = availability.TempUnschedulableUntil
		}

		account[acc.ID] = item
	}

	return platform, group, account, &collectedAt, nil
}

type OpsAccountAvailability struct {
	Group       *GroupAvailability
	Accounts    map[int64]*AccountAvailability
	CollectedAt *time.Time
}

func (s *OpsService) GetAccountAvailability(ctx context.Context, platformFilter string, groupIDFilter *int64) (*OpsAccountAvailability, error) {
	if s == nil {
		return nil, errors.New("ops service is nil")
	}

	if s.getAccountAvailability != nil {
		return s.getAccountAvailability(ctx, platformFilter, groupIDFilter)
	}

	_, groupStats, accountStats, collectedAt, err := s.GetAccountAvailabilityStats(ctx, platformFilter, groupIDFilter)
	if err != nil {
		return nil, err
	}

	var group *GroupAvailability
	if groupIDFilter != nil && *groupIDFilter > 0 {
		group = groupStats[*groupIDFilter]
	}

	if accountStats == nil {
		accountStats = map[int64]*AccountAvailability{}
	}

	return &OpsAccountAvailability{
		Group:       group,
		Accounts:    accountStats,
		CollectedAt: collectedAt,
	}, nil
}
