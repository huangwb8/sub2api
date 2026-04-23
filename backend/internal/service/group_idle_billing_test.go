//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseAndFormatClockTimeSeconds(t *testing.T) {
	seconds, err := ParseClockTimeToSeconds("07:08:09")
	require.NoError(t, err)
	require.Equal(t, 7*3600+8*60+9, seconds)
	require.Equal(t, "07:08:09", FormatClockTimeSeconds(seconds))
}

func TestGroupIdleBillingActiveAtSupportsCrossDayWindow(t *testing.T) {
	start := 23 * 3600
	end := 2 * 3600
	multiplier := 0.6
	group := &Group{
		IdleRateMultiplier: &multiplier,
		IdleStartSeconds:   &start,
		IdleEndSeconds:     &end,
	}

	require.True(t, group.IsIdleBillingActiveAt(time.Date(2026, 4, 23, 23, 30, 0, 0, beijingLocation)))
	require.True(t, group.IsIdleBillingActiveAt(time.Date(2026, 4, 24, 1, 30, 0, 0, beijingLocation)))
	require.False(t, group.IsIdleBillingActiveAt(time.Date(2026, 4, 24, 12, 0, 0, 0, beijingLocation)))
}

func TestGroupResolveIdleBillingOverrides(t *testing.T) {
	baseProfit := 90.0
	idleProfit := 25.0
	idleMultiplier := 0.4
	start := 0
	end := 12 * 3600
	group := &Group{
		RateMultiplier:             1.5,
		IdleRateMultiplier:         &idleMultiplier,
		ExtraProfitRatePercent:     &baseProfit,
		IdleExtraProfitRatePercent: &idleProfit,
		IdleStartSeconds:           &start,
		IdleEndSeconds:             &end,
	}

	activeAt := time.Date(2026, 4, 23, 8, 0, 0, 0, beijingLocation)
	inactiveAt := time.Date(2026, 4, 23, 20, 0, 0, 0, beijingLocation)

	require.Equal(t, idleMultiplier, group.ResolveRateMultiplierAt(activeAt, 1.8))
	require.Equal(t, 1.8, group.ResolveRateMultiplierAt(inactiveAt, 1.8))
	require.Equal(t, idleProfit, *group.ResolveExtraProfitRateAt(activeAt))
	require.Equal(t, baseProfit, *group.ResolveExtraProfitRateAt(inactiveAt))
}

func TestValidateIdleBillingConfigRequiresCompleteWindowAndPricing(t *testing.T) {
	start := 0
	err := validateIdleBillingConfig(&Group{IdleStartSeconds: &start})
	require.EqualError(t, err, "idle_start_time and idle_end_time must be set together")

	end := 0
	err = validateIdleBillingConfig(&Group{IdleStartSeconds: &start, IdleEndSeconds: &end, IdleRateMultiplier: f64p(0.8)})
	require.EqualError(t, err, "idle_start_time and idle_end_time cannot be the same")

	end = 3600
	err = validateIdleBillingConfig(&Group{IdleStartSeconds: &start, IdleEndSeconds: &end})
	require.EqualError(t, err, "idle billing requires idle_rate_multiplier or idle_extra_profit_rate_percent")
}
