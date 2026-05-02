package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		snapshot, snapshotErr := r.loadExistingChargeSnapshot(ctx, tx, cmd.RequestID, cmd.APIKeyID)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		return &service.UsageBillingApplyResult{Applied: false, ChargeSnapshot: snapshot}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (
			request_id,
			api_key_id,
			request_fingerprint,
			balance_cost_cny,
			fx_rate_usd_cny,
			fx_rate_source,
			fx_fetched_at,
			fx_safety_margin
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint, nullableFloat64(cmd.BalanceCostCNY), nullableFloat64(cmd.FXRateUSDCNY), nullableString(cmd.FXRateSource), nullableTime(cmd.FXFetchedAt), nullableFloat64(cmd.FXSafetyMargin)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, cmd.RequestID, cmd.APIKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, cmd.RequestID, cmd.APIKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCostUSD > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCostUSD); err != nil {
			return err
		}
	}

	if cmd.BalanceCostCNY > 0 {
		if err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCostCNY, cmd.MaxBalanceOverdraftCNY); err != nil {
			return err
		}
	}

	if cmd.APIKeyQuotaCostUSD > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCostUSD)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCostUSD > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCostUSD); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCostUSD > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeChatAPI) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		if err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCostUSD); err != nil {
			return err
		}
	}

	result.ChargeSnapshot = &service.UsageChargeSnapshot{
		ChargedAmountCNY: cmd.BalanceCostCNY,
		FXRateUSDCNY:     cmd.FXRateUSDCNY,
		FXRateSource:     cmd.FXRateSource,
		FXFetchedAt:      cmd.FXFetchedAt,
		FXSafetyMargin:   cmd.FXSafetyMargin,
	}

	return nil
}

func nullableFloat64(value float64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func (r *usageBillingRepository) loadExistingChargeSnapshot(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64) (*service.UsageChargeSnapshot, error) {
	snapshot, err := queryUsageBillingChargeSnapshot(ctx, tx, `
		SELECT balance_cost_cny, fx_rate_usd_cny, fx_rate_source, fx_fetched_at, fx_safety_margin
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID)
	if err == nil {
		return snapshot, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	snapshot, err = queryUsageBillingChargeSnapshot(ctx, tx, `
		SELECT balance_cost_cny, fx_rate_usd_cny, fx_rate_source, fx_fetched_at, fx_safety_margin
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func queryUsageBillingChargeSnapshot(ctx context.Context, tx *sql.Tx, query string, requestID string, apiKeyID int64) (*service.UsageChargeSnapshot, error) {
	var chargedAmount sql.NullFloat64
	var fxRate sql.NullFloat64
	var fxSource sql.NullString
	var fxFetchedAt sql.NullTime
	var fxSafetyMargin sql.NullFloat64
	if err := tx.QueryRowContext(ctx, query, requestID, apiKeyID).Scan(&chargedAmount, &fxRate, &fxSource, &fxFetchedAt, &fxSafetyMargin); err != nil {
		return nil, err
	}
	if !chargedAmount.Valid && !fxRate.Valid && !fxSource.Valid && !fxFetchedAt.Valid && !fxSafetyMargin.Valid {
		return nil, nil
	}
	snapshot := &service.UsageChargeSnapshot{}
	if chargedAmount.Valid {
		snapshot.ChargedAmountCNY = chargedAmount.Float64
	}
	if fxRate.Valid {
		snapshot.FXRateUSDCNY = fxRate.Float64
	}
	if fxSource.Valid {
		snapshot.FXRateSource = fxSource.String
	}
	if fxFetchedAt.Valid {
		fetchedAt := fxFetchedAt.Time.UTC()
		snapshot.FXFetchedAt = &fetchedAt
	}
	if fxSafetyMargin.Valid {
		snapshot.FXSafetyMargin = fxSafetyMargin.Float64
	}
	return snapshot, nil
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1::numeric,
			weekly_usage_usd = us.weekly_usage_usd + $1::numeric,
			monthly_usage_usd = us.monthly_usage_usd + $1::numeric,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64, maxOverdraftCNY float64) error {
	if maxOverdraftCNY < 0 {
		maxOverdraftCNY = 0
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE users
		SET balance = balance - $1::numeric,
			updated_at = NOW()
		WHERE id = $2
			AND deleted_at IS NULL
			AND balance - $1::numeric >= -$3::numeric
	`, amount, userID, maxOverdraftCNY)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM users
			WHERE id = $1 AND deleted_at IS NULL
		)
	`, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return service.ErrUserNotFound
	}
	return service.ErrInsufficientBalance
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1::numeric,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1::numeric >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1::numeric < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1::numeric ELSE usage_5h + $1::numeric END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1::numeric ELSE usage_1d + $1::numeric END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1::numeric ELSE usage_7d + $1::numeric END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) error {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1::numeric)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN COALESCE((extra->>'quota_daily_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '24 hours'::interval <= NOW()
					THEN $1::numeric
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1::numeric END,
					'quota_daily_start',
					CASE WHEN COALESCE((extra->>'quota_daily_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '24 hours'::interval <= NOW()
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN COALESCE((extra->>'quota_weekly_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '168 hours'::interval <= NOW()
					THEN $1::numeric
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1::numeric END,
					'quota_weekly_start',
					CASE WHEN COALESCE((extra->>'quota_weekly_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '168 hours'::interval <= NOW()
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var newUsed, limit float64
	if rows.Next() {
		if err := rows.Scan(&newUsed, &limit); err != nil {
			return err
		}
	} else {
		if err := rows.Err(); err != nil {
			return err
		}
		return service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if limit > 0 && newUsed >= limit && (newUsed-amount) < limit {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return err
		}
	}
	return nil
}
