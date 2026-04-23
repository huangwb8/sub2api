package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	secondsPerDay          = 24 * 60 * 60
	beijingUTCOffsetSecond = 8 * 60 * 60
)

var beijingLocation = time.FixedZone("Asia/Shanghai", beijingUTCOffsetSecond)

// ParseClockTimeToSeconds parses HH:MM[:SS] into seconds-of-day in Beijing time.
func ParseClockTimeToSeconds(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("time of day is required")
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format %q, expected HH:MM or HH:MM:SS", value)
	}

	parsePart := func(part string, name string, min int, max int) (int, error) {
		if len(part) != 2 {
			return 0, fmt.Errorf("invalid %s in %q", name, value)
		}
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return 0, fmt.Errorf("invalid %s in %q", name, value)
		}
		if parsed < min || parsed > max {
			return 0, fmt.Errorf("%s out of range in %q", name, value)
		}
		return parsed, nil
	}

	hour, err := parsePart(parts[0], "hour", 0, 23)
	if err != nil {
		return 0, err
	}
	minute, err := parsePart(parts[1], "minute", 0, 59)
	if err != nil {
		return 0, err
	}
	second := 0
	if len(parts) == 3 {
		second, err = parsePart(parts[2], "second", 0, 59)
		if err != nil {
			return 0, err
		}
	}

	return hour*3600 + minute*60 + second, nil
}

func FormatClockTimeSeconds(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	if seconds >= secondsPerDay {
		seconds %= secondsPerDay
	}

	hour := seconds / 3600
	minute := (seconds % 3600) / 60
	second := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

func IsValidClockTimeSeconds(seconds int) bool {
	return seconds >= 0 && seconds < secondsPerDay
}

func (g *Group) HasIdleBillingWindow() bool {
	if g == nil || g.IdleStartSeconds == nil || g.IdleEndSeconds == nil {
		return false
	}
	if !IsValidClockTimeSeconds(*g.IdleStartSeconds) || !IsValidClockTimeSeconds(*g.IdleEndSeconds) {
		return false
	}
	return *g.IdleStartSeconds != *g.IdleEndSeconds
}

func (g *Group) HasIdleBillingConfigured() bool {
	if !g.HasIdleBillingWindow() {
		return false
	}
	return g.HasIdleRateMultiplierConfigured() || g.HasIdleExtraProfitRateConfigured()
}

func (g *Group) IsIdleBillingActiveAt(now time.Time) bool {
	if !g.HasIdleBillingConfigured() {
		return false
	}

	localNow := now.In(beijingLocation)
	currentSeconds := localNow.Hour()*3600 + localNow.Minute()*60 + localNow.Second()
	start := *g.IdleStartSeconds
	end := *g.IdleEndSeconds

	if start < end {
		return currentSeconds >= start && currentSeconds < end
	}
	return currentSeconds >= start || currentSeconds < end
}

func (g *Group) ResolveRateMultiplierAt(now time.Time, fallback float64) float64 {
	if g != nil && g.IsIdleBillingActiveAt(now) && g.IdleRateMultiplier != nil {
		return *g.IdleRateMultiplier
	}
	return fallback
}

func (g *Group) ResolveExtraProfitRateAt(now time.Time) *float64 {
	if g != nil && g.IsIdleBillingActiveAt(now) && g.IdleExtraProfitRatePercent != nil {
		return g.IdleExtraProfitRatePercent
	}
	if g == nil {
		return nil
	}
	return g.ExtraProfitRatePercent
}

func validateIdleBillingConfig(group *Group) error {
	if group == nil {
		return nil
	}

	if group.IdleStartSeconds != nil && !IsValidClockTimeSeconds(*group.IdleStartSeconds) {
		return fmt.Errorf("idle_start_time must be within 00:00:00 to 23:59:59")
	}
	if group.IdleEndSeconds != nil && !IsValidClockTimeSeconds(*group.IdleEndSeconds) {
		return fmt.Errorf("idle_end_time must be within 00:00:00 to 23:59:59")
	}
	if group.IdleRateMultiplier != nil && *group.IdleRateMultiplier < 0 {
		return fmt.Errorf("idle_rate_multiplier must be >= 0")
	}
	if group.IdleExtraProfitRatePercent != nil && *group.IdleExtraProfitRatePercent < 0 {
		return fmt.Errorf("idle_extra_profit_rate_percent must be >= 0")
	}

	hasWindowField := group.IdleStartSeconds != nil || group.IdleEndSeconds != nil
	hasPricingField := group.IdleRateMultiplier != nil || group.IdleExtraProfitRatePercent != nil
	if !hasWindowField && !hasPricingField {
		return nil
	}
	if group.IdleStartSeconds == nil || group.IdleEndSeconds == nil {
		return fmt.Errorf("idle_start_time and idle_end_time must be set together")
	}
	if *group.IdleStartSeconds == *group.IdleEndSeconds {
		return fmt.Errorf("idle_start_time and idle_end_time cannot be the same")
	}
	if !hasPricingField {
		return fmt.Errorf("idle billing requires idle_rate_multiplier or idle_extra_profit_rate_percent")
	}

	return nil
}
