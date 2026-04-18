package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetProfitabilityTrend_UsesSchemaDriftSafeNumericCasting(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(
		"CASE\\s+WHEN\\s+NULLIF\\(BTRIM\\(ul\\.charged_amount_cny::text\\), ''\\) IS NULL THEN 0::numeric[\\s\\S]*"+
			"NULLIF\\(BTRIM\\(ul\\.estimated_cost_cny::text\\), ''\\)[\\s\\S]*"+
			"NULLIF\\(BTRIM\\(po\\.amount::text\\), ''\\)",
	).
		WithArgs(
			start,
			end,
			"subscription",
			service.OrderStatusCompleted,
			service.OrderStatusPaid,
			service.OrderStatusRecharging,
		).
		WillReturnRows(
			sqlmock.NewRows([]string{
				"date",
				"revenue_balance_cny",
				"revenue_subscription_cny",
				"estimated_cost_cny",
			}),
		)

	trend, err := repo.GetProfitabilityTrend(context.Background(), start, end, "day")
	require.NoError(t, err)
	require.Empty(t, trend)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetProfitabilityTrend_ComputesProfitAndRate(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("WITH balance_usage AS").
		WithArgs(
			start,
			end,
			"subscription",
			service.OrderStatusCompleted,
			service.OrderStatusPaid,
			service.OrderStatusRecharging,
		).
		WillReturnRows(
			sqlmock.NewRows([]string{
				"date",
				"revenue_balance_cny",
				"revenue_subscription_cny",
				"estimated_cost_cny",
			}).AddRow("2026-04-18", 10.5, 5.25, 3.5),
		)

	trend, err := repo.GetProfitabilityTrend(context.Background(), start, end, "day")
	require.NoError(t, err)
	require.Len(t, trend, 1)
	require.Equal(t, "2026-04-18", trend[0].Date)
	require.Equal(t, 10.5, trend[0].RevenueBalanceCNY)
	require.Equal(t, 5.25, trend[0].RevenueSubscriptionCNY)
	require.Equal(t, 3.5, trend[0].EstimatedCostCNY)
	require.Equal(t, 12.25, trend[0].ProfitCNY)
	if assertRate := trend[0].ExtraProfitRatePercent; assertRate == nil {
		t.Fatalf("expected extra profit rate percent")
	} else {
		require.Equal(t, 350.0, *assertRate)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
