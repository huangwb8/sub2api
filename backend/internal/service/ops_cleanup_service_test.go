//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsCleanupPlan(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	cutoff, truncate, ok := opsCleanupPlan(now, -1)
	require.False(t, ok)
	require.False(t, truncate)
	require.True(t, cutoff.IsZero())

	cutoff, truncate, ok = opsCleanupPlan(now, 0)
	require.True(t, ok)
	require.True(t, truncate)
	require.True(t, cutoff.IsZero())

	cutoff, truncate, ok = opsCleanupPlan(now, 7)
	require.True(t, ok)
	require.False(t, truncate)
	require.Equal(t, now.AddDate(0, 0, -7), cutoff)
}

func TestOpsCleanupServiceRunCleanupOnce_TruncatesWhenRetentionZero(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	for _, table := range []string{
		"ops_error_logs",
		"ops_retry_attempts",
		"ops_alert_events",
		"ops_system_logs",
		"ops_system_log_cleanup_audits",
		"ops_system_metrics",
		"ops_metrics_hourly",
		"ops_metrics_daily",
	} {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM " + table).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
		mock.ExpectExec("TRUNCATE TABLE " + table).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}

	svc := &OpsCleanupService{
		db: db,
		cfg: &config.Config{
			Ops: config.OpsConfig{
				Cleanup: config.OpsCleanupConfig{
					ErrorLogRetentionDays:      0,
					MinuteMetricsRetentionDays: 0,
					HourlyMetricsRetentionDays: 0,
				},
			},
		},
	}

	counts, err := svc.runCleanupOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), counts.errorLogs)
	require.Equal(t, int64(2), counts.systemMetrics)
	require.Equal(t, int64(2), counts.dailyPreagg)
	require.NoError(t, mock.ExpectationsWereMet())
}
