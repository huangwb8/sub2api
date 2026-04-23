package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type upgradeSettingRepoStub struct {
	values map[string]string
}

func (s *upgradeSettingRepoStub) Get(_ context.Context, key string) (*service.Setting, error) {
	return &service.Setting{Key: key, Value: s.values[key]}, nil
}

func (s *upgradeSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *upgradeSettingRepoStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *upgradeSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s *upgradeSettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		s.values[key] = value
	}
	return nil
}

func (s *upgradeSettingRepoStub) GetAll(_ context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}

func (s *upgradeSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func newSubscriptionUpgradeServiceTestEnv(t *testing.T, enabledTypes string) (*service.SubscriptionUpgradeService, *dbent.Client) {
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

	userRepo := repository.NewUserRepository(client, db)
	groupRepo := repository.NewGroupRepository(client, db)
	userSubRepo := repository.NewUserSubscriptionRepository(client)
	subscriptionSvc := service.NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	configSvc := service.NewPaymentConfigService(client, &upgradeSettingRepoStub{
		values: map[string]string{
			service.SettingPaymentEnabled:      "true",
			service.SettingEnabledPaymentTypes: enabledTypes,
			service.SettingLoadBalanceStrategy: payment.DefaultLoadBalanceStrategy,
		},
	}, nil)

	return service.NewSubscriptionUpgradeService(client, subscriptionSvc, configSvc, userRepo), client
}

type upgradeLoadBalancerStub struct{}

func (upgradeLoadBalancerStub) GetInstanceConfig(context.Context, int64) (map[string]string, error) {
	return map[string]string{}, nil
}

func (upgradeLoadBalancerStub) SelectInstance(context.Context, payment.PaymentType, []string, payment.Strategy, float64) (*payment.InstanceSelection, error) {
	return nil, nil
}

func mustCreateUpgradeUser(t *testing.T, ctx context.Context, client *dbent.Client, balance float64) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("upgrade-%s@example.com", strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")))).
		SetUsername("upgrade-user").
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetBalance(balance).
		SetConcurrency(1).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func mustCreateUpgradeGroup(t *testing.T, ctx context.Context, client *dbent.Client, name string) *dbent.Group {
	t.Helper()
	group, err := client.Group.Create().
		SetName(name).
		SetPlatform(service.PlatformOpenAI).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	return group
}

func mustCreateUpgradePlan(t *testing.T, ctx context.Context, client *dbent.Client, groupID int64, name string, price float64, family string, rank int) *dbent.SubscriptionPlan {
	t.Helper()
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(groupID).
		SetName(name).
		SetDescription(name + " description").
		SetPrice(price).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(true).
		SetUpgradeFamily(family).
		SetUpgradeRank(rank).
		Save(ctx)
	require.NoError(t, err)
	return plan
}

func TestSubscriptionUpgradeService_ListUpgradeOptions_FiltersAndCalculatesQuote(t *testing.T) {
	t.Parallel()

	svc, client := newSubscriptionUpgradeServiceTestEnv(t, "balance,wxpay")
	ctx := context.Background()
	now := time.Now().UTC()

	user := mustCreateUpgradeUser(t, ctx, client, 20)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "basic-group")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "pro-group")
	lowerRankGroup := mustCreateUpgradeGroup(t, ctx, client, "lower-group")
	otherFamilyGroup := mustCreateUpgradeGroup(t, ctx, client, "other-group")
	conflictGroup := mustCreateUpgradeGroup(t, ctx, client, "conflict-group")

	sourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic", 100, "openai-team", 10)
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 20)
	mustCreateUpgradePlan(t, ctx, client, lowerRankGroup.ID, "Starter", 80, "openai-team", 5)
	mustCreateUpgradePlan(t, ctx, client, otherFamilyGroup.ID, "Other", 160, "anthropic-team", 30)
	mustCreateUpgradePlan(t, ctx, client, conflictGroup.ID, "Conflict", 180, "openai-team", 30)

	billingStartedAt := now.AddDate(0, 0, -15)
	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(sourcePlan.ID).
		SetCurrentPlanName(sourcePlan.Name).
		SetCurrentPlanPriceCny(sourcePlan.Price).
		SetCurrentPlanValidityDays(sourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(sourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(billingStartedAt).
		SetStartsAt(billingStartedAt).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(billingStartedAt).
		SetNotes("active basic").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(conflictGroup.ID).
		SetStartsAt(now.AddDate(0, 0, -2)).
		SetExpiresAt(now.AddDate(0, 0, 28)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.AddDate(0, 0, -2)).
		SetNotes("conflict active sub").
		Save(ctx)
	require.NoError(t, err)

	result, err := svc.ListUpgradeOptions(ctx, user.ID, sourceSub.ID)
	require.NoError(t, err)
	require.Equal(t, sourceGroup.ID, result.SourceGroupID)
	require.Equal(t, sourcePlan.ID, result.SourcePlanID)
	require.Equal(t, sourcePlan.Name, result.SourcePlanName)
	require.InDelta(t, 0.5, result.RemainingRatio, 0.0001)
	require.InDelta(t, 50, result.CreditCNY, 0.0001)
	require.Len(t, result.Options, 1)
	require.Equal(t, targetPlan.ID, result.Options[0].TargetPlanID)
	require.Equal(t, targetGroup.ID, result.Options[0].TargetGroupID)
	require.InDelta(t, 100, result.Options[0].PayableCNY, 0.0001)
	require.Equal(t, payment.TypeWxpay, result.Options[0].DefaultPaymentType)
}

func TestSubscriptionUpgradeService_BuildUpgradeQuote_RequiresPlanSnapshot(t *testing.T) {
	t.Parallel()

	svc, client := newSubscriptionUpgradeServiceTestEnv(t, "balance,wxpay")
	ctx := context.Background()
	now := time.Now().UTC()

	user := mustCreateUpgradeUser(t, ctx, client, 200)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "legacy-group")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "modern-group")
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 120, "openai-team", 20)

	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetStartsAt(now.AddDate(0, 0, -10)).
		SetExpiresAt(now.AddDate(0, 0, 20)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.AddDate(0, 0, -10)).
		SetNotes("legacy subscription").
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.BuildUpgradeQuote(ctx, user.ID, sourceSub.ID, targetPlan.ID, now)
	require.Error(t, err)
	require.Contains(t, err.Error(), "plan snapshot")
}

func TestSubscriptionUpgradeService_ListUpgradeOptions_UsesSameGroupUpgradeMetadataFallback(t *testing.T) {
	t.Parallel()

	svc, client := newSubscriptionUpgradeServiceTestEnv(t, "balance,wxpay")
	ctx := context.Background()
	now := time.Now().UTC()

	user := mustCreateUpgradeUser(t, ctx, client, 20)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "legacy-basic-group")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "modern-pro-group")

	legacySourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Legacy", 100, "", 0)
	currentSourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Current", 110, "openai-team", 10)
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 20)

	billingStartedAt := now.AddDate(0, 0, -15)
	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(legacySourcePlan.ID).
		SetCurrentPlanName(legacySourcePlan.Name).
		SetCurrentPlanPriceCny(legacySourcePlan.Price).
		SetCurrentPlanValidityDays(legacySourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(legacySourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(billingStartedAt).
		SetStartsAt(billingStartedAt).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(billingStartedAt).
		SetNotes("legacy basic subscription").
		Save(ctx)
	require.NoError(t, err)

	result, err := svc.ListUpgradeOptions(ctx, user.ID, sourceSub.ID)
	require.NoError(t, err)
	require.Len(t, result.Options, 1)
	require.Equal(t, targetPlan.ID, result.Options[0].TargetPlanID)
	require.Equal(t, currentSourcePlan.UpgradeFamily, result.Options[0].UpgradeFamily)
	require.Equal(t, 10, currentSourcePlan.UpgradeRank)
}

func TestSubscriptionUpgradeService_ListUpgradeOptions_RejectsAmbiguousSameGroupUpgradeMetadata(t *testing.T) {
	t.Parallel()

	svc, client := newSubscriptionUpgradeServiceTestEnv(t, "balance,wxpay")
	ctx := context.Background()
	now := time.Now().UTC()

	user := mustCreateUpgradeUser(t, ctx, client, 20)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "ambiguous-basic-group")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "ambiguous-pro-group")

	legacySourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Legacy", 100, "", 0)
	mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Current A", 110, "openai-team", 10)
	mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Current B", 120, "openai-enterprise", 20)
	mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 30)

	billingStartedAt := now.AddDate(0, 0, -15)
	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(legacySourcePlan.ID).
		SetCurrentPlanName(legacySourcePlan.Name).
		SetCurrentPlanPriceCny(legacySourcePlan.Price).
		SetCurrentPlanValidityDays(legacySourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(legacySourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(billingStartedAt).
		SetStartsAt(billingStartedAt).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(billingStartedAt).
		SetNotes("legacy basic subscription with ambiguous metadata").
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.ListUpgradeOptions(ctx, user.ID, sourceSub.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inconsistent")
}

func TestSubscriptionUpgradeService_BuildUpgradeQuote_UsesSameGroupMetadataFallbackWhenSourcePlanIsMissing(t *testing.T) {
	t.Parallel()

	svc, client := newSubscriptionUpgradeServiceTestEnv(t, "balance,wxpay")
	ctx := context.Background()
	now := time.Now().UTC()

	user := mustCreateUpgradeUser(t, ctx, client, 20)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "deleted-source-group")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "deleted-target-group")

	legacySourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Legacy", 100, "", 0)
	mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Current", 120, "openai-team", 10)
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 20)

	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(legacySourcePlan.ID).
		SetCurrentPlanName(legacySourcePlan.Name).
		SetCurrentPlanPriceCny(legacySourcePlan.Price).
		SetCurrentPlanValidityDays(legacySourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(legacySourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(now.AddDate(0, 0, -15)).
		SetStartsAt(now.AddDate(0, 0, -15)).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.AddDate(0, 0, -15)).
		SetNotes("legacy basic subscription with deleted source plan").
		Save(ctx)
	require.NoError(t, err)

	err = client.SubscriptionPlan.DeleteOneID(legacySourcePlan.ID).Exec(ctx)
	require.NoError(t, err)

	quote, err := svc.BuildUpgradeQuote(ctx, user.ID, sourceSub.ID, targetPlan.ID, now)
	require.NoError(t, err)
	require.Equal(t, targetPlan.ID, quote.TargetPlanID)
	require.InDelta(t, 100, quote.PayableCNY, 0.0001)
}

func TestSubscriptionUpgradeService_BuildUpgradeQuote_RejectsWhenNoSameGroupUpgradeMetadataExists(t *testing.T) {
	t.Parallel()

	svc, client := newSubscriptionUpgradeServiceTestEnv(t, "balance,wxpay")
	ctx := context.Background()
	now := time.Now().UTC()

	user := mustCreateUpgradeUser(t, ctx, client, 20)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "unsupported-source-group")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "unsupported-target-group")

	sourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Legacy", 100, "", 0)
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 20)

	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(sourcePlan.ID).
		SetCurrentPlanName(sourcePlan.Name).
		SetCurrentPlanPriceCny(sourcePlan.Price).
		SetCurrentPlanValidityDays(sourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(sourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(now.AddDate(0, 0, -15)).
		SetStartsAt(now.AddDate(0, 0, -15)).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.AddDate(0, 0, -15)).
		SetNotes("legacy basic subscription without upgrade metadata").
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.BuildUpgradeQuote(ctx, user.ID, sourceSub.ID, targetPlan.ID, now)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not enabled for this plan")
}

func TestPaymentService_CreateOrder_WithBalanceSubscriptionUpgradeCompletes_ForLegacyPlanMetadataFallback(t *testing.T) {
	t.Parallel()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	settingRepo := &upgradeSettingRepoStub{
		values: map[string]string{
			service.SettingPaymentEnabled:      "true",
			service.SettingEnabledPaymentTypes: "balance,wxpay",
			service.SettingLoadBalanceStrategy: payment.DefaultLoadBalanceStrategy,
		},
	}
	userRepo := repository.NewUserRepository(client, db)
	groupRepo := repository.NewGroupRepository(client, db)
	userSubRepo := repository.NewUserSubscriptionRepository(client)
	subscriptionSvc := service.NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	configSvc := service.NewPaymentConfigService(client, settingRepo, nil)
	paymentSvc := service.NewPaymentService(client, payment.NewRegistry(), upgradeLoadBalancerStub{}, nil, subscriptionSvc, configSvc, userRepo, groupRepo)

	ctx := context.Background()
	now := time.Now().UTC()
	user := mustCreateUpgradeUser(t, ctx, client, 100)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "upgrade-legacy-basic")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "upgrade-modern-pro")
	legacySourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Legacy", 100, "", 0)
	mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic Current", 120, "openai-team", 10)
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 20)

	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(legacySourcePlan.ID).
		SetCurrentPlanName(legacySourcePlan.Name).
		SetCurrentPlanPriceCny(legacySourcePlan.Price).
		SetCurrentPlanValidityDays(legacySourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(legacySourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(now.AddDate(0, 0, -15)).
		SetStartsAt(now.AddDate(0, 0, -15)).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.AddDate(0, 0, -15)).
		SetNotes("legacy basic subscription").
		Save(ctx)
	require.NoError(t, err)

	resp, err := paymentSvc.CreateOrder(ctx, service.CreateOrderRequest{
		UserID:               user.ID,
		Amount:               999,
		PaymentType:          payment.TypeBalance,
		OrderType:            payment.OrderTypeSubscriptionUpgrade,
		PlanID:               targetPlan.ID,
		SourceSubscriptionID: sourceSub.ID,
	})
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusCompleted, resp.Status)

	activeTargetSub, err := subscriptionSvc.GetActiveSubscription(ctx, user.ID, targetGroup.ID)
	require.NoError(t, err)
	require.NotNil(t, activeTargetSub.CurrentPlanID)
	require.Equal(t, targetPlan.ID, *activeTargetSub.CurrentPlanID)
}

func TestPaymentService_CreateOrder_WithBalanceSubscriptionUpgradeCompletes(t *testing.T) {
	t.Parallel()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	settingRepo := &upgradeSettingRepoStub{
		values: map[string]string{
			service.SettingPaymentEnabled:      "true",
			service.SettingEnabledPaymentTypes: "balance,wxpay",
			service.SettingLoadBalanceStrategy: payment.DefaultLoadBalanceStrategy,
		},
	}
	userRepo := repository.NewUserRepository(client, db)
	groupRepo := repository.NewGroupRepository(client, db)
	userSubRepo := repository.NewUserSubscriptionRepository(client)
	subscriptionSvc := service.NewSubscriptionService(groupRepo, userSubRepo, nil, client, nil)
	configSvc := service.NewPaymentConfigService(client, settingRepo, nil)
	paymentSvc := service.NewPaymentService(client, payment.NewRegistry(), upgradeLoadBalancerStub{}, nil, subscriptionSvc, configSvc, userRepo, groupRepo)

	ctx := context.Background()
	now := time.Now().UTC()
	user := mustCreateUpgradeUser(t, ctx, client, 100)
	sourceGroup := mustCreateUpgradeGroup(t, ctx, client, "upgrade-basic")
	targetGroup := mustCreateUpgradeGroup(t, ctx, client, "upgrade-pro")
	sourcePlan := mustCreateUpgradePlan(t, ctx, client, sourceGroup.ID, "Basic", 100, "openai-team", 10)
	targetPlan := mustCreateUpgradePlan(t, ctx, client, targetGroup.ID, "Pro", 150, "openai-team", 20)

	sourceSub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(sourceGroup.ID).
		SetCurrentPlanID(sourcePlan.ID).
		SetCurrentPlanName(sourcePlan.Name).
		SetCurrentPlanPriceCny(sourcePlan.Price).
		SetCurrentPlanValidityDays(sourcePlan.ValidityDays).
		SetCurrentPlanValidityUnit(sourcePlan.ValidityUnit).
		SetBillingCycleStartedAt(now.AddDate(0, 0, -15)).
		SetStartsAt(now.AddDate(0, 0, -15)).
		SetExpiresAt(now.AddDate(0, 0, 15)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now.AddDate(0, 0, -15)).
		SetNotes("current basic subscription").
		Save(ctx)
	require.NoError(t, err)

	resp, err := paymentSvc.CreateOrder(ctx, service.CreateOrderRequest{
		UserID:               user.ID,
		Amount:               999,
		PaymentType:          payment.TypeBalance,
		OrderType:            payment.OrderTypeSubscriptionUpgrade,
		PlanID:               targetPlan.ID,
		SourceSubscriptionID: sourceSub.ID,
	})
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusCompleted, resp.Status)
	require.InDelta(t, 100, resp.Amount, 0.0001)
	require.InDelta(t, 100, resp.PayAmount, 0.0001)

	order, err := client.PaymentOrder.Get(ctx, resp.OrderID)
	require.NoError(t, err)
	require.Equal(t, payment.OrderTypeSubscriptionUpgrade, order.OrderType)
	require.NotNil(t, order.SourceSubscriptionID)
	require.Equal(t, sourceSub.ID, *order.SourceSubscriptionID)
	require.NotNil(t, order.UpgradeCreditCny)
	require.InDelta(t, 50, *order.UpgradeCreditCny, 0.0001)
	require.NotNil(t, order.UpgradePayableCny)
	require.InDelta(t, 100, *order.UpgradePayableCny, 0.0001)

	updatedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 0, updatedUser.Balance, 0.0001)

	updatedSource, err := client.UserSubscription.Get(ctx, sourceSub.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusExpired, updatedSource.Status)
	require.False(t, updatedSource.ExpiresAt.After(time.Now()))

	activeTargetSub, err := subscriptionSvc.GetActiveSubscription(ctx, user.ID, targetGroup.ID)
	require.NoError(t, err)
	require.NotNil(t, activeTargetSub.CurrentPlanID)
	require.Equal(t, targetPlan.ID, *activeTargetSub.CurrentPlanID)
	require.Equal(t, targetPlan.Name, activeTargetSub.CurrentPlanName)
}
