package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type proxyProbeLogRepository struct {
	db *sql.DB
}

func NewProxyProbeLogRepository(client *dbent.Client, db *sql.DB) service.ProxyProbeLogRepository {
	return &proxyProbeLogRepository{db: db}
}

func (r *proxyProbeLogRepository) Create(ctx context.Context, input service.ProxyProbeLogInput) error {
	if r == nil || r.db == nil {
		return nil
	}
	input = service.NormalizeProxyProbeLogInput(input)
	if input.ProxyID <= 0 {
		return nil
	}

	var latency any
	if input.LatencyMs != nil {
		latency = *input.LatencyMs
	}
	var errorMessage any
	if input.ErrorMessage != "" {
		errorMessage = input.ErrorMessage
	}
	var ip, countryCode, country, region, city any
	if input.ExitInfo != nil {
		ip = nullableTrimmedString(input.ExitInfo.IP)
		countryCode = nullableTrimmedString(input.ExitInfo.CountryCode)
		country = nullableTrimmedString(input.ExitInfo.Country)
		region = nullableTrimmedString(input.ExitInfo.Region)
		city = nullableTrimmedString(input.ExitInfo.City)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO proxy_probe_logs (
			proxy_id, source, target, success, latency_ms, error_message,
			ip_address, country_code, country, region, city, checked_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())
	`, input.ProxyID, input.Source, input.Target, input.Success, latency, errorMessage,
		ip, countryCode, country, region, city, input.CheckedAt.UTC())
	return err
}

func (r *proxyProbeLogRepository) List(ctx context.Context, query service.ProxyProbeLogQuery) ([]service.ProxyProbeLog, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("proxy probe log repository not ready")
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	where := "proxy_id = $1"
	args := []any{query.ProxyID}
	if !query.Since.IsZero() {
		args = append(args, query.Since.UTC())
		where += fmt.Sprintf(" AND checked_at >= $%d", len(args))
	}
	if !query.Until.IsZero() {
		args = append(args, query.Until.UTC())
		where += fmt.Sprintf(" AND checked_at <= $%d", len(args))
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM proxy_probe_logs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, proxy_id, source, target, success, latency_ms, error_message,
		       ip_address, country_code, country, region, city, checked_at, created_at
		  FROM proxy_probe_logs
		 WHERE `+where+`
		 ORDER BY checked_at DESC, id DESC
		 LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close() //nolint:errcheck

	logs := make([]service.ProxyProbeLog, 0, limit)
	for rows.Next() {
		log, scanErr := scanProxyProbeLog(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	pageSize := limit
	page := offset/limit + 1
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return logs, &pagination.PaginationResult{Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (r *proxyProbeLogRepository) GetLast(ctx context.Context, proxyID int64) (*service.ProxyProbeLog, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("proxy probe log repository not ready")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, proxy_id, source, target, success, latency_ms, error_message,
		       ip_address, country_code, country, region, city, checked_at, created_at
		  FROM proxy_probe_logs
		 WHERE proxy_id = $1
		 ORDER BY checked_at DESC, id DESC
		 LIMIT 1
	`, proxyID)
	log, err := scanProxyProbeLog(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *proxyProbeLogRepository) GetReliability(ctx context.Context, proxyID int64, now time.Time) (*service.ProxyReliabilityReport, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("proxy probe log repository not ready")
	}
	if now.IsZero() {
		now = time.Now()
	}
	report := &service.ProxyReliabilityReport{
		ProxyID:     proxyID,
		GeneratedAt: now.UTC(),
		InterpretationNotes: []string{
			"巡检失败是风险信号，不等同于真实请求必失败。",
			"usage_success_count 统计窗口内已成功写入 usage_logs 的请求数。",
		},
	}
	last, err := r.GetLast(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	report.LastProbe = last

	for _, window := range []struct {
		label string
		hours int
	}{{"24h", 24}, {"7d", 24 * 7}} {
		item, err := r.loadReliabilityWindow(ctx, proxyID, now.Add(-time.Duration(window.hours)*time.Hour), now, window.label, window.hours)
		if err != nil {
			return nil, err
		}
		report.Windows = append(report.Windows, item)
	}
	for _, minutes := range []int{15, 30, 60} {
		item, err := r.loadFailureFollowup(ctx, proxyID, now.Add(-7*24*time.Hour), minutes)
		if err != nil {
			return nil, err
		}
		report.FailureFollowups = append(report.FailureFollowups, item)
	}
	return report, nil
}

func (r *proxyProbeLogRepository) DeleteBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if r == nil || r.db == nil || cutoff.IsZero() {
		return 0, nil
	}
	if limit <= 0 {
		limit = 5000
	}
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM proxy_probe_logs
		 WHERE id IN (
			SELECT id FROM proxy_probe_logs
			 WHERE checked_at < $1
			 ORDER BY checked_at ASC
			 LIMIT $2
		 )
	`, cutoff.UTC(), limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *proxyProbeLogRepository) loadReliabilityWindow(ctx context.Context, proxyID int64, since, until time.Time, label string, hours int) (service.ProxyReliabilityWindow, error) {
	item := service.ProxyReliabilityWindow{Label: label, Hours: hours}
	var lastFailureAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE success) AS successful,
		       MAX(checked_at) FILTER (WHERE NOT success) AS last_failure_at
		  FROM proxy_probe_logs
		 WHERE proxy_id = $1 AND checked_at >= $2 AND checked_at <= $3
	`, proxyID, since.UTC(), until.UTC()).Scan(&item.ProbeTotal, &item.ProbeSuccess, &lastFailureAt)
	if err != nil {
		return item, err
	}
	if lastFailureAt.Valid {
		item.LastProbeFailureAt = &lastFailureAt.Time
	}
	if item.ProbeTotal > 0 {
		rate := float64(item.ProbeSuccess) / float64(item.ProbeTotal)
		item.ProbeSuccessRate = &rate
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM usage_logs
		 WHERE proxy_id = $1 AND created_at >= $2 AND created_at <= $3
	`, proxyID, since.UTC(), until.UTC()).Scan(&item.UsageSuccessCount); err != nil {
		return item, err
	}
	return item, nil
}

func (r *proxyProbeLogRepository) loadFailureFollowup(ctx context.Context, proxyID int64, since time.Time, minutes int) (service.ProxyReliabilityFollowup, error) {
	item := service.ProxyReliabilityFollowup{Minutes: minutes}
	err := r.db.QueryRowContext(ctx, `
		WITH failures AS (
			SELECT checked_at
			  FROM proxy_probe_logs
			 WHERE proxy_id = $1 AND success = FALSE AND checked_at >= $2
		)
		SELECT COUNT(*) AS failed_probe_count,
		       COALESCE(SUM((
		       	SELECT COUNT(*)
		       	  FROM usage_logs ul
		       	 WHERE ul.proxy_id = $1
		       	   AND ul.created_at > failures.checked_at
		       	   AND ul.created_at <= failures.checked_at + ($3::text || ' minutes')::interval
		       )), 0) AS usage_success_count
		  FROM failures
	`, proxyID, since.UTC(), minutes).Scan(&item.FailedProbeCount, &item.UsageSuccessCount)
	return item, err
}

type proxyProbeLogScanner interface {
	Scan(dest ...any) error
}

func scanProxyProbeLog(scanner proxyProbeLogScanner) (service.ProxyProbeLog, error) {
	var log service.ProxyProbeLog
	var latency sql.NullInt64
	var errMsg, ip, countryCode, country, region, city sql.NullString
	err := scanner.Scan(
		&log.ID, &log.ProxyID, &log.Source, &log.Target, &log.Success, &latency, &errMsg,
		&ip, &countryCode, &country, &region, &city, &log.CheckedAt, &log.CreatedAt,
	)
	if err != nil {
		return log, err
	}
	log.LatencyMs = proxyProbeNullInt64Ptr(latency)
	log.ErrorMessage = proxyProbeNullStringPtr(errMsg)
	log.IPAddress = proxyProbeNullStringPtr(ip)
	log.CountryCode = proxyProbeNullStringPtr(countryCode)
	log.Country = proxyProbeNullStringPtr(country)
	log.Region = proxyProbeNullStringPtr(region)
	log.City = proxyProbeNullStringPtr(city)
	return log, nil
}

func nullableTrimmedString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func proxyProbeNullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func proxyProbeNullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}
