package service

import (
	"context"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Plan CRUD ---

// PlanGroupInfo holds the group details needed for subscription plan display.
type PlanGroupInfo struct {
	Platform                   string   `json:"platform"`
	Name                       string   `json:"name"`
	RateMultiplier             float64  `json:"rate_multiplier"`
	IdleRateMultiplier         *float64 `json:"idle_rate_multiplier,omitempty"`
	IdleExtraProfitRatePercent *float64 `json:"idle_extra_profit_rate_percent,omitempty"`
	IdleStartTime              *string  `json:"idle_start_time,omitempty"`
	IdleEndTime                *string  `json:"idle_end_time,omitempty"`
	DailyLimitUSD              *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD             *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD            *float64 `json:"monthly_limit_usd"`
	ModelScopes                []string `json:"supported_model_scopes"`
}

// GetGroupPlatformMap returns a map of group_id → platform for the given plans.
func (s *PaymentConfigService) GetGroupPlatformMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]string {
	info := s.GetGroupInfoMap(ctx, plans)
	m := make(map[int64]string, len(info))
	for id, gi := range info {
		m[id] = gi.Platform
	}
	return m
}

// GetGroupInfoMap returns a map of group_id → PlanGroupInfo for the given plans.
func (s *PaymentConfigService) GetGroupInfoMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]PlanGroupInfo {
	ids := make([]int64, 0, len(plans))
	seen := make(map[int64]bool)
	for _, p := range plans {
		if !seen[p.GroupID] {
			seen[p.GroupID] = true
			ids = append(ids, p.GroupID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	groups, err := s.entClient.Group.Query().Where(group.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil
	}
	m := make(map[int64]PlanGroupInfo, len(groups))
	for _, g := range groups {
		var idleStartTime *string
		var idleEndTime *string
		if g.IdleStartSeconds != nil {
			formatted := FormatClockTimeSeconds(*g.IdleStartSeconds)
			idleStartTime = &formatted
		}
		if g.IdleEndSeconds != nil {
			formatted := FormatClockTimeSeconds(*g.IdleEndSeconds)
			idleEndTime = &formatted
		}
		m[int64(g.ID)] = PlanGroupInfo{
			Platform:                   g.Platform,
			Name:                       g.Name,
			RateMultiplier:             g.RateMultiplier,
			IdleRateMultiplier:         g.IdleRateMultiplier,
			IdleExtraProfitRatePercent: g.IdleExtraProfitRatePercent,
			IdleStartTime:              idleStartTime,
			IdleEndTime:                idleEndTime,
			DailyLimitUSD:              g.DailyLimitUsd,
			WeeklyLimitUSD:             g.WeeklyLimitUsd,
			MonthlyLimitUSD:            g.MonthlyLimitUsd,
			ModelScopes:                g.SupportedModelScopes,
		}
	}
	return m
}

func (s *PaymentConfigService) ListPlans(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	return s.entClient.SubscriptionPlan.Query().Order(subscriptionplan.BySortOrder()).All(ctx)
}

func (s *PaymentConfigService) ListPlansForSale(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	groupIDs, err := s.listSellableSubscriptionGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return []*dbent.SubscriptionPlan{}, nil
	}
	return s.entClient.SubscriptionPlan.Query().
		Where(subscriptionplan.ForSaleEQ(true), subscriptionplan.GroupIDIn(groupIDs...)).
		Order(subscriptionplan.BySortOrder()).
		All(ctx)
}

func (s *PaymentConfigService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := s.validatePlanGroup(ctx, req.GroupID); err != nil {
		return nil, err
	}
	if err := validatePlanPrice(req.Price); err != nil {
		return nil, err
	}
	validityUnit, err := normalizePlanValidityUnit(req.ValidityUnit)
	if err != nil {
		return nil, infraerrors.BadRequest("PLAN_VALIDITY_UNIT_INVALID", err.Error())
	}
	upgradeFamily, upgradeRank, err := normalizePlanUpgradeMetadata(req.UpgradeFamily, req.UpgradeRank)
	if err != nil {
		return nil, err
	}
	b := s.entClient.SubscriptionPlan.Create().
		SetGroupID(req.GroupID).SetName(req.Name).SetDescription(req.Description).
		SetPrice(req.Price).SetValidityDays(req.ValidityDays).SetValidityUnit(validityUnit).
		SetFeatures(req.Features).SetProductName(req.ProductName).
		SetUpgradeFamily(upgradeFamily).SetUpgradeRank(upgradeRank).
		SetForSale(req.ForSale).SetSortOrder(req.SortOrder)
	if req.OriginalPrice != nil {
		b.SetOriginalPrice(*req.OriginalPrice)
	}
	return b.Save(ctx)
}

// UpdatePlan updates a subscription plan by ID (patch semantics).
// NOTE: This function exceeds 30 lines due to per-field nil-check patch update boilerplate.
func (s *PaymentConfigService) UpdatePlan(ctx context.Context, id int64, req UpdatePlanRequest) (*dbent.SubscriptionPlan, error) {
	existing, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	}
	if req.GroupID != nil {
		if err := s.validatePlanGroup(ctx, *req.GroupID); err != nil {
			return nil, err
		}
	}
	if req.Price != nil {
		if err := validatePlanPrice(*req.Price); err != nil {
			return nil, err
		}
	}
	if req.ValidityUnit != nil {
		validityUnit, err := normalizePlanValidityUnit(*req.ValidityUnit)
		if err != nil {
			return nil, infraerrors.BadRequest("PLAN_VALIDITY_UNIT_INVALID", err.Error())
		}
		req.ValidityUnit = &validityUnit
	}
	upgradeFamily := existing.UpgradeFamily
	upgradeRank := existing.UpgradeRank
	if req.UpgradeFamily != nil {
		upgradeFamily = *req.UpgradeFamily
	}
	if req.UpgradeRank != nil {
		upgradeRank = *req.UpgradeRank
	}
	normalizedUpgradeFamily, normalizedUpgradeRank, err := normalizePlanUpgradeMetadata(upgradeFamily, upgradeRank)
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin plan update transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	u := tx.SubscriptionPlan.UpdateOneID(id)
	if req.GroupID != nil {
		u.SetGroupID(*req.GroupID)
	}
	if req.Name != nil {
		u.SetName(*req.Name)
	}
	if req.Description != nil {
		u.SetDescription(*req.Description)
	}
	if req.Price != nil {
		u.SetPrice(*req.Price)
	}
	if req.OriginalPrice != nil {
		u.SetOriginalPrice(*req.OriginalPrice)
	}
	if req.ValidityDays != nil {
		u.SetValidityDays(*req.ValidityDays)
	}
	if req.ValidityUnit != nil {
		u.SetValidityUnit(*req.ValidityUnit)
	}
	if req.Features != nil {
		u.SetFeatures(*req.Features)
	}
	if req.ProductName != nil {
		u.SetProductName(*req.ProductName)
	}
	if req.UpgradeFamily != nil || req.UpgradeRank != nil {
		u.SetUpgradeFamily(normalizedUpgradeFamily)
		u.SetUpgradeRank(normalizedUpgradeRank)
	}
	if req.ForSale != nil {
		u.SetForSale(*req.ForSale)
	}
	if req.SortOrder != nil {
		u.SetSortOrder(*req.SortOrder)
	}

	updated, err := u.Save(ctx)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if _, err := tx.UserSubscription.Update().
			Where(usersubscription.CurrentPlanIDEQ(id)).
			SetCurrentPlanName(updated.Name).
			Save(ctx); err != nil {
			return nil, fmt.Errorf("sync subscription plan names: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit plan update transaction: %w", err)
	}
	committed = true

	return updated, nil
}

func (s *PaymentConfigService) DeletePlan(ctx context.Context, id int64) error {
	count, err := s.countPendingOrdersByPlan(ctx, id)
	if err != nil {
		return fmt.Errorf("check pending orders: %w", err)
	}
	if count > 0 {
		return infraerrors.Conflict("PENDING_ORDERS",
			fmt.Sprintf("this plan has %d in-progress orders and cannot be deleted — wait for orders to complete first", count))
	}
	return s.entClient.SubscriptionPlan.DeleteOneID(id).Exec(ctx)
}

// GetPlan returns a subscription plan by ID.
func (s *PaymentConfigService) GetPlan(ctx context.Context, id int64) (*dbent.SubscriptionPlan, error) {
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	}
	return plan, nil
}

func (s *PaymentConfigService) listSellableSubscriptionGroupIDs(ctx context.Context) ([]int64, error) {
	groups, err := s.entClient.Group.Query().
		Where(
			group.StatusEQ(StatusActive),
			group.SubscriptionTypeEQ(SubscriptionTypeSubscription),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query sellable plan groups: %w", err)
	}
	ids := make([]int64, 0, len(groups))
	for _, grp := range groups {
		ids = append(ids, grp.ID)
	}
	return ids, nil
}

func (s *PaymentConfigService) validatePlanGroup(ctx context.Context, groupID int64) error {
	grp, err := s.entClient.Group.Get(ctx, groupID)
	if err != nil {
		return infraerrors.NotFound("PLAN_GROUP_NOT_FOUND", "subscription plan group not found")
	}
	if grp.Status != StatusActive {
		return infraerrors.BadRequest("PLAN_GROUP_INACTIVE", "subscription plan group must be active")
	}
	if grp.SubscriptionType != SubscriptionTypeSubscription {
		return infraerrors.BadRequest("PLAN_GROUP_NOT_SUBSCRIPTION", "subscription plan group must use subscription billing")
	}
	return nil
}

func validatePlanPrice(price float64) error {
	if price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "subscription plan price must be greater than 0")
	}
	return nil
}

func normalizePlanUpgradeMetadata(family string, rank int) (string, int, error) {
	family = strings.TrimSpace(family)
	if family == "" {
		return "", 0, nil
	}
	if rank < 0 {
		return "", 0, infraerrors.BadRequest("PLAN_UPGRADE_RANK_INVALID", "subscription plan upgrade rank must be greater than or equal to 0")
	}
	return family, rank, nil
}
