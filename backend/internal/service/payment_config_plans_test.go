package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newPaymentConfigServiceSQLite(t *testing.T) (*PaymentConfigService, *dbent.Client) {
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

	return NewPaymentConfigService(client, nil, nil), client
}

func mustCreatePlanGroup(t *testing.T, ctx context.Context, client *dbent.Client, name, status, subscriptionType string) *dbent.Group {
	t.Helper()

	group, err := client.Group.Create().
		SetName(name).
		SetPlatform(PlatformOpenAI).
		SetStatus(status).
		SetSubscriptionType(subscriptionType).
		Save(ctx)
	require.NoError(t, err)
	return group
}

func mustCreateSubscriptionPlan(t *testing.T, ctx context.Context, client *dbent.Client, groupID int64, name string, forSale bool) *dbent.SubscriptionPlan {
	t.Helper()

	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(groupID).
		SetName(name).
		SetDescription(name + " description").
		SetPrice(19.9).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetForSale(forSale).
		Save(ctx)
	require.NoError(t, err)
	return plan
}

func TestPaymentConfigService_ListPlansForSale_FiltersInvalidGroups(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	activeSubGroup := mustCreatePlanGroup(t, ctx, client, "active-sub", StatusActive, SubscriptionTypeSubscription)
	disabledSubGroup := mustCreatePlanGroup(t, ctx, client, "disabled-sub", StatusDisabled, SubscriptionTypeSubscription)
	activeStandardGroup := mustCreatePlanGroup(t, ctx, client, "active-standard", StatusActive, SubscriptionTypeStandard)

	activePlan := mustCreateSubscriptionPlan(t, ctx, client, activeSubGroup.ID, "valid-plan", true)
	mustCreateSubscriptionPlan(t, ctx, client, disabledSubGroup.ID, "disabled-group-plan", true)
	mustCreateSubscriptionPlan(t, ctx, client, activeStandardGroup.ID, "wrong-type-plan", true)
	mustCreateSubscriptionPlan(t, ctx, client, activeSubGroup.ID, "not-for-sale", false)

	plans, err := svc.ListPlansForSale(ctx)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, activePlan.ID, plans[0].ID)
	require.Equal(t, "valid-plan", plans[0].Name)
}

func TestPaymentConfigService_CreatePlan_RejectsInvalidGroups(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	activeSubGroup := mustCreatePlanGroup(t, ctx, client, "active-sub", StatusActive, SubscriptionTypeSubscription)
	disabledSubGroup := mustCreatePlanGroup(t, ctx, client, "disabled-sub", StatusDisabled, SubscriptionTypeSubscription)
	activeStandardGroup := mustCreatePlanGroup(t, ctx, client, "active-standard", StatusActive, SubscriptionTypeStandard)

	_, err := svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:      activeStandardGroup.ID,
		Name:         "wrong-type",
		Description:  "wrong-type",
		Price:        9.9,
		ValidityDays: 30,
		ValidityUnit: "day",
		ForSale:      true,
	})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "subscription") || strings.Contains(err.Error(), "订阅"))

	_, err = svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:      disabledSubGroup.ID,
		Name:         "disabled-group",
		Description:  "disabled-group",
		Price:        9.9,
		ValidityDays: 30,
		ValidityUnit: "day",
		ForSale:      true,
	})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "active") || strings.Contains(err.Error(), "可用"))

	plan, err := svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:       activeSubGroup.ID,
		Name:          "valid-plan",
		Description:   "valid-plan",
		Price:         9.9,
		ValidityDays:  30,
		ValidityUnit:  "days",
		ForSale:       true,
		UpgradeFamily: "openai-team",
		UpgradeRank:   20,
	})
	require.NoError(t, err)
	require.Equal(t, activeSubGroup.ID, plan.GroupID)
	require.Equal(t, planValidityUnitDay, plan.ValidityUnit)
	require.Equal(t, "openai-team", plan.UpgradeFamily)
	require.Equal(t, 20, plan.UpgradeRank)
}

func TestPaymentConfigService_CreatePlan_RejectsNonPositivePrice(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	activeSubGroup := mustCreatePlanGroup(t, ctx, client, "active-sub", StatusActive, SubscriptionTypeSubscription)

	for _, price := range []float64{0, -9.9} {
		_, err := svc.CreatePlan(ctx, CreatePlanRequest{
			GroupID:      activeSubGroup.ID,
			Name:         fmt.Sprintf("invalid-price-%v", price),
			Description:  "invalid price",
			Price:        price,
			ValidityDays: 30,
			ValidityUnit: "day",
			ForSale:      true,
		})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "price") || strings.Contains(err.Error(), "价格"))
	}
}

func TestPaymentConfigService_UpdatePlan_RejectsInvalidGroupTransitions(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	activeSubGroup := mustCreatePlanGroup(t, ctx, client, "active-sub", StatusActive, SubscriptionTypeSubscription)
	disabledSubGroup := mustCreatePlanGroup(t, ctx, client, "disabled-sub", StatusDisabled, SubscriptionTypeSubscription)
	activeStandardGroup := mustCreatePlanGroup(t, ctx, client, "active-standard", StatusActive, SubscriptionTypeStandard)
	plan := mustCreateSubscriptionPlan(t, ctx, client, activeSubGroup.ID, "valid-plan", true)

	_, err := svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{GroupID: &activeStandardGroup.ID})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "subscription") || strings.Contains(err.Error(), "订阅"))

	_, err = svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{GroupID: &disabledSubGroup.ID})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "active") || strings.Contains(err.Error(), "可用"))

	newName := "updated-name"
	updated, err := svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name)
	require.Equal(t, activeSubGroup.ID, updated.GroupID)
}

func TestPaymentConfigService_UpdatePlan_NormalizesPluralValidityUnit(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	activeSubGroup := mustCreatePlanGroup(t, ctx, client, "active-sub", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, activeSubGroup.ID, "valid-plan", true)

	weeks := "weeks"
	family := "openai-team"
	rank := 30
	updated, err := svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{
		ValidityUnit:  &weeks,
		UpgradeFamily: &family,
		UpgradeRank:   &rank,
	})
	require.NoError(t, err)
	require.Equal(t, planValidityUnitWeek, updated.ValidityUnit)
	require.Equal(t, family, updated.UpgradeFamily)
	require.Equal(t, rank, updated.UpgradeRank)
}

func TestPaymentConfigService_UpdatePlan_RejectsNonPositivePrice(t *testing.T) {
	t.Parallel()

	svc, client := newPaymentConfigServiceSQLite(t)
	ctx := context.Background()

	activeSubGroup := mustCreatePlanGroup(t, ctx, client, "active-sub", StatusActive, SubscriptionTypeSubscription)
	plan := mustCreateSubscriptionPlan(t, ctx, client, activeSubGroup.ID, "valid-plan", true)

	for _, price := range []float64{0, -19.9} {
		price := price
		_, err := svc.UpdatePlan(ctx, plan.ID, UpdatePlanRequest{Price: &price})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "price") || strings.Contains(err.Error(), "价格"))
	}
}
