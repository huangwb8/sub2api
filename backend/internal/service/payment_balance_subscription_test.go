//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
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

	userRepo := &paymentTestUserRepo{client: client}
	groupRepo := &paymentTestGroupRepo{client: client}
	userSubRepo := &paymentTestUserSubscriptionRepo{client: client}
	subscriptionSvc := NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	redeemRepo := &redeemRepoStub{}
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
	require.NotNil(t, sub.CurrentPlanID)
	require.Equal(t, plan.ID, *sub.CurrentPlanID)
	require.Equal(t, plan.Name, sub.CurrentPlanName)
	require.NotNil(t, sub.CurrentPlanPriceCNY)
	require.InDelta(t, plan.Price, *sub.CurrentPlanPriceCNY, 0.0001)
	require.NotNil(t, sub.CurrentPlanValidityDays)
	require.Equal(t, plan.ValidityDays, *sub.CurrentPlanValidityDays)
	require.Equal(t, plan.ValidityUnit, sub.CurrentPlanValidityUnit)
	require.NotNil(t, sub.BillingCycleStartedAt)
}

func TestPaymentService_RetryFulfillment_RecoversBalanceSubscriptionInRechargingWithoutSecondDebit(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()

	user := mustCreatePaymentUser(t, ctx, client, 100)
	group := mustCreatePlanGroup(t, ctx, client, "balance-recovery-group", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "balance-recovery-plan", true)
	cfg, err := svc.configService.GetPaymentConfig(ctx)
	require.NoError(t, err)
	serviceUser, err := svc.userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)

	order, err := svc.createBalanceSubscriptionOrderInTx(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      plan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      plan.ID,
	}, serviceUser, plan, cfg, plan.Price)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRecharging, order.Status)

	afterDebit, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, user.Balance-plan.Price, afterDebit.Balance, 0.0001)

	err = svc.RetryFulfillment(ctx, order.ID)
	require.NoError(t, err)

	completed, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completed.Status)

	afterRetry, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, afterDebit.Balance, afterRetry.Balance, 0.0001, "retry must not deduct wallet balance again")

	sub, err := svc.subscriptionSvc.GetActiveSubscription(ctx, user.ID, group.ID)
	require.NoError(t, err)
	require.NotNil(t, sub)
}

func TestPaymentService_CreateOrder_WithBalancePaymentInvalidatesBalanceCache(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()
	cache := &billingCacheWorkerStub{balance: 100}
	billingCacheSvc := NewBillingCacheService(cache, svc.userRepo, svc.subscriptionSvc.userSubRepo, nil, nil)
	t.Cleanup(billingCacheSvc.Stop)
	svc.subscriptionSvc.billingCacheService = billingCacheSvc

	user := mustCreatePaymentUser(t, ctx, client, 100)
	group := mustCreatePlanGroup(t, ctx, client, "balance-cache-group", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "balance-cache-plan", true)

	resp, err := svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      plan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      plan.ID,
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, resp.Status)
	require.GreaterOrEqual(t, atomic.LoadInt64(&cache.balanceInvalidates), int64(1))
}

func TestPaymentService_CreateOrder_WithBalancePaymentRefreshesPlanSnapshotOnRenewal(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()

	user := mustCreatePaymentUser(t, ctx, client, 500)
	group := mustCreatePlanGroup(t, ctx, client, "renew-sub-group", StatusActive, SubscriptionTypeSubscription)
	firstPlan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "basic-plan", true)
	secondPlan, err := client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("pro-plan").
		SetDescription("pro plan").
		SetPrice(39.9).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      firstPlan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      firstPlan.ID,
	})
	require.NoError(t, err)

	firstSub, err := svc.subscriptionSvc.GetActiveSubscription(ctx, user.ID, group.ID)
	require.NoError(t, err)
	require.NotNil(t, firstSub.BillingCycleStartedAt)
	firstCycleStart := *firstSub.BillingCycleStartedAt

	time.Sleep(10 * time.Millisecond)

	_, err = svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      secondPlan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      secondPlan.ID,
	})
	require.NoError(t, err)

	renewedSub, err := svc.subscriptionSvc.GetActiveSubscription(ctx, user.ID, group.ID)
	require.NoError(t, err)
	require.NotNil(t, renewedSub.CurrentPlanID)
	require.Equal(t, secondPlan.ID, *renewedSub.CurrentPlanID)
	require.Equal(t, secondPlan.Name, renewedSub.CurrentPlanName)
	require.NotNil(t, renewedSub.CurrentPlanPriceCNY)
	require.InDelta(t, secondPlan.Price, *renewedSub.CurrentPlanPriceCNY, 0.0001)
	require.NotNil(t, renewedSub.CurrentPlanValidityDays)
	require.Equal(t, secondPlan.ValidityDays, *renewedSub.CurrentPlanValidityDays)
	require.Equal(t, secondPlan.ValidityUnit, renewedSub.CurrentPlanValidityUnit)
	require.NotNil(t, renewedSub.BillingCycleStartedAt)
	require.True(t, renewedSub.BillingCycleStartedAt.After(firstCycleStart) || renewedSub.BillingCycleStartedAt.Equal(firstCycleStart))
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

func TestPaymentService_CreateOrder_WithBalancePaymentBypassesDailyRechargeLimit(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()
	settingRepo := svc.configService.settingRepo.(*paymentSettingRepoStub)
	settingRepo.values[SettingDailyRechargeLimit] = "1"

	user := mustCreatePaymentUser(t, ctx, client, 100)
	group := mustCreatePlanGroup(t, ctx, client, "daily-limit-sub-group", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, group.ID, "daily-limit-plan", true)

	resp, err := svc.CreateOrder(ctx, CreateOrderRequest{
		UserID:      user.ID,
		Amount:      plan.Price,
		PaymentType: payment.TypeBalance,
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      plan.ID,
	})
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, resp.Status)
}

func TestPaymentService_CheckDailyLimit_IgnoresSubscriptionOrders(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentServiceSQLite(t)
	ctx := context.Background()
	user := mustCreatePaymentUser(t, ctx, client, 100)

	_, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(999).
		SetPayAmount(999).
		SetFeeRate(0).
		SetRechargeCode("PAY-sub-only").
		SetOutTradeNo("sub2_subscription_only").
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("trade-sub-only").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now()).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = svc.checkDailyLimit(ctx, tx, user.ID, 10, 50)
	require.NoError(t, err)
}
