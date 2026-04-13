//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type paymentSettingRepoStub struct {
	values map[string]string
}

func (s *paymentSettingRepoStub) Get(_ context.Context, key string) (*Setting, error) {
	return &Setting{Key: key, Value: s.values[key]}, nil
}

func (s *paymentSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *paymentSettingRepoStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *paymentSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s *paymentSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *paymentSettingRepoStub) GetAll(_ context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}

func (s *paymentSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func newPaymentServiceSQLite(t *testing.T) (*PaymentService, *dbent.Client) {
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

	settingRepo := &paymentSettingRepoStub{
		values: map[string]string{
			SettingPaymentEnabled:      "true",
			SettingLoadBalanceStrategy: payment.DefaultLoadBalanceStrategy,
		},
	}

	userRepo := repository.NewUserRepository(client, db)
	groupRepo := repository.NewGroupRepository(client, db)
	userSubRepo := repository.NewUserSubscriptionRepository(client)
	subscriptionSvc := NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	redeemRepo := repository.NewRedeemCodeRepository(client)
	redeemSvc := NewRedeemService(redeemRepo, userRepo, subscriptionSvc, nil, nil, client, nil)
	configSvc := NewPaymentConfigService(client, settingRepo, nil)

	return NewPaymentService(client, payment.NewRegistry(), stubPaymentLoadBalancer{}, redeemSvc, subscriptionSvc, configSvc, userRepo, groupRepo), client
}

func mustCreatePaymentUser(t *testing.T, ctx context.Context, client *dbent.Client, balance float64) *dbent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("user-%s@example.com", strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")))).
		SetUsername("payment-user").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetBalance(balance).
		SetConcurrency(1).
		SetStatus(StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func TestPaymentService_CreateOrder_WithBalancePaymentCompletesSubscription(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()

	user := mustCreatePaymentUser(t, ctx, client, 100)
	group := mustCreatePlanGroup(t, ctx, client, "balance-sub-group", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "balance-plan", true)

	resp, err := svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      plan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      plan.ID,
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, resp.Status)
	require.Equal(t, payment.TypeBalance, resp.PaymentType)
	require.Equal(t, plan.Price, resp.PayAmount)

	order, err := client.PaymentOrder.Get(ctx, resp.OrderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, order.Status)
	require.Equal(t, payment.TypeBalance, order.PaymentType)
	require.NotNil(t, order.CompletedAt)

	updatedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, user.Balance-plan.Price, updatedUser.Balance, 0.0001)

	sub, err := svc.subscriptionSvc.GetActiveSubscription(ctx, user.ID, group.ID)
	require.NoError(t, err)
	require.NotNil(t, sub)
}

func TestPaymentService_CreateOrder_WithBalancePaymentRejectsInsufficientBalance(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()

	user := mustCreatePaymentUser(t, ctx, client, 5)
	group := mustCreatePlanGroup(t, ctx, client, "insufficient-sub-group", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "expensive-plan", true)

	_, err := svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      plan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      plan.ID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "INSUFFICIENT_BALANCE")

	orderCount, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, orderCount)

	updatedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, user.Balance, updatedUser.Balance, 0.0001)
}

func TestPaymentService_ExecuteRefund_RecreditsBalanceForBalancePaidSubscription(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()

	user := mustCreatePaymentUser(t, ctx, client, 100)
	group := mustCreatePlanGroup(t, ctx, client, "refund-sub-group", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "refund-plan", true)

	orderResp, err := svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      plan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      plan.ID,
	})
	require.NoError(t, err)

	refundPlan, earlyResult, err := svc.PrepareRefund(ctx, orderResp.OrderID, 0, "test refund", true, true)
	require.NoError(t, err)
	require.Nil(t, earlyResult)

	result, err := svc.ExecuteRefund(ctx, refundPlan)
	require.NoError(t, err)
	require.True(t, result.Success)

	updatedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, user.Balance, updatedUser.Balance, 0.0001)

	order, err := client.PaymentOrder.Get(ctx, orderResp.OrderID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, order.Status)

	_, err = svc.subscriptionSvc.GetActiveSubscription(ctx, user.ID, group.ID)
	require.Error(t, err)
}
