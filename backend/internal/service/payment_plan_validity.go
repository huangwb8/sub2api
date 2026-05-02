package service

import (
	"fmt"
	"strings"
)

const (
	planValidityUnitDay   = "day"
	planValidityUnitWeek  = "week"
	planValidityUnitMonth = "month"
	planValidityUnitYear  = "year"
)

var planValidityUnitAliases = map[string]string{
	"":       planValidityUnitDay,
	"day":    planValidityUnitDay,
	"days":   planValidityUnitDay,
	"week":   planValidityUnitWeek,
	"weeks":  planValidityUnitWeek,
	"month":  planValidityUnitMonth,
	"months": planValidityUnitMonth,
	"year":   planValidityUnitYear,
	"years":  planValidityUnitYear,
}

func normalizePlanValidityUnit(unit string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(unit))
	if canonical, ok := planValidityUnitAliases[normalized]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("unsupported validity unit %q (allowed: day/week/month/year)", unit)
}
