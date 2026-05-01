//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newTemporaryInvitationServiceSQLite(t *testing.T) (*TemporaryInvitationService, *dbent.Client) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return NewTemporaryInvitationService(client, nil, time.Minute), client
}

func mustCreateTemporaryInvitationUser(t *testing.T, ctx context.Context, client *dbent.Client, email string, status string, deadline *time.Time, disabledAt *time.Time, deleteAt *time.Time) *dbent.User {
	t.Helper()

	builder := client.User.Create().
		SetEmail(email).
		SetUsername("temp-user").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetBalance(0).
		SetConcurrency(1).
		SetStatus(status).
		SetTemporaryInvitation(true)
	if deadline != nil {
		builder = builder.SetTemporaryInvitationDeadlineAt(*deadline)
	}
	if disabledAt != nil {
		builder = builder.SetTemporaryInvitationDisabledAt(*disabledAt)
	}
	if deleteAt != nil {
		builder = builder.SetTemporaryInvitationDeleteAt(*deleteAt)
	}
	user, err := builder.Save(ctx)
	require.NoError(t, err)
	return user
}

func mustCreateQualifiedPaymentOrder(t *testing.T, ctx context.Context, client *dbent.Client, u *dbent.User, amount float64) {
	t.Helper()

	_, err := client.PaymentOrder.Create().
		SetUserID(u.ID).
		SetUserEmail(u.Email).
		SetUserName(u.Username).
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode("TEMP-QUALIFY").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-no-temp").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("test.local").
		Save(ctx)
	require.NoError(t, err)
}

func TestTemporaryInvitationService_RunOnce_DisablesExpiredUsersWithoutQualifiedRecharge(t *testing.T) {
	t.Parallel()

	svc, client := newTemporaryInvitationServiceSQLite(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	deadline := now.Add(-time.Minute)
	u := mustCreateTemporaryInvitationUser(t, ctx, client, "disable-temp@test.com", StatusActive, &deadline, nil, nil)

	result, err := svc.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, result.Disabled)
	require.Equal(t, 0, result.Normalized)
	require.Equal(t, 0, result.Deleted)

	updated, err := client.User.Get(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDisabled, updated.Status)
	require.NotNil(t, updated.TemporaryInvitationDisabledAt)
	require.NotNil(t, updated.TemporaryInvitationDeleteAt)
	require.WithinDuration(t, now, *updated.TemporaryInvitationDisabledAt, time.Second)
	require.WithinDuration(t, now.Add(TemporaryInvitationDeleteWindow), *updated.TemporaryInvitationDeleteAt, time.Second)
}

func TestTemporaryInvitationService_RunOnce_NormalizesQualifiedUsers(t *testing.T) {
	t.Parallel()

	svc, client := newTemporaryInvitationServiceSQLite(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	deadline := now.Add(-2 * time.Hour)
	u := mustCreateTemporaryInvitationUser(t, ctx, client, "qualified-temp@test.com", StatusActive, &deadline, nil, nil)
	mustCreateQualifiedPaymentOrder(t, ctx, client, u, 31)

	result, err := svc.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, result.Normalized)
	require.Equal(t, 0, result.Disabled)
	require.Equal(t, 0, result.Deleted)

	updated, err := client.User.Get(ctx, u.ID)
	require.NoError(t, err)
	require.False(t, updated.TemporaryInvitation)
	require.Nil(t, updated.TemporaryInvitationDeadlineAt)
	require.Nil(t, updated.TemporaryInvitationDisabledAt)
	require.Nil(t, updated.TemporaryInvitationDeleteAt)
	require.Equal(t, StatusActive, updated.Status)
}

func TestTemporaryInvitationService_RunOnce_HardDeletesUsersPastDeleteWindow(t *testing.T) {
	t.Parallel()

	svc, client := newTemporaryInvitationServiceSQLite(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	deadline := now.Add(-8 * 24 * time.Hour)
	disabledAt := now.Add(-8 * 24 * time.Hour)
	deleteAt := now.Add(-time.Minute)
	u := mustCreateTemporaryInvitationUser(t, ctx, client, "delete-temp@test.com", StatusDisabled, &deadline, &disabledAt, &deleteAt)

	_, err := client.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS refresh_tokens (
		token_hash TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_version INTEGER NOT NULL DEFAULT 0,
		family_id TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL
	)`)
	require.NoError(t, err)

	_, err = client.ExecContext(ctx, "INSERT INTO refresh_tokens (token_hash, user_id, token_version, family_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)", "hash-temp", u.ID, 0, "family-temp", now, now.Add(time.Hour))
	require.NoError(t, err)

	result, err := svc.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, result.Deleted)
	require.Equal(t, 0, result.Normalized)
	require.Equal(t, 0, result.Disabled)

	_, err = client.User.Query().Where(dbuser.IDEQ(u.ID)).Only(mixins.SkipSoftDelete(ctx))
	require.True(t, dbent.IsNotFound(err))

	rows, err := client.QueryContext(ctx, "SELECT COUNT(1) FROM refresh_tokens WHERE user_id = ?", u.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var count int
	require.NoError(t, rows.Scan(&count))
	require.Equal(t, 0, count)
}
