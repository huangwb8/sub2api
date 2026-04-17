package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UpgradeOption struct {
	TargetPlanID       int64   `json:"target_plan_id"`
	TargetGroupID      int64   `json:"target_group_id"`
	TargetPlanName     string  `json:"target_plan_name"`
	TargetPriceCNY     float64 `json:"target_price_cny"`
	DefaultPaymentType string  `json:"default_payment_type"`
	PayableCNY         float64 `json:"payable_cny"`
	UpgradeFamily      string  `json:"upgrade_family"`
	UpgradeRank        int     `json:"upgrade_rank"`
}

type UpgradeOptionsResult struct {
	SourceSubscriptionID int64           `json:"source_subscription_id"`
	SourceGroupID        int64           `json:"source_group_id"`
	SourcePlanID         int64           `json:"source_plan_id"`
	SourcePlanName       string          `json:"source_plan_name"`
	RemainingRatio       float64         `json:"remaining_ratio"`
	CreditCNY            float64         `json:"credit_cny"`
	Options              []UpgradeOption `json:"options"`
}

type UpgradeQuote struct {
	SourceSubscriptionID int64   `json:"source_subscription_id"`
	SourceGroupID        int64   `json:"source_group_id"`
	SourcePlanID         int64   `json:"source_plan_id"`
	SourcePlanName       string  `json:"source_plan_name"`
	TargetPlanID         int64   `json:"target_plan_id"`
	TargetGroupID        int64   `json:"target_group_id"`
	TargetPlanName       string  `json:"target_plan_name"`
	TargetPriceCNY       float64 `json:"target_price_cny"`
	RemainingRatio       float64 `json:"remaining_ratio"`
	CreditCNY            float64 `json:"credit_cny"`
	PayableCNY           float64 `json:"payable_cny"`
	DefaultPaymentType   string  `json:"default_payment_type"`
	UpgradeFamily        string  `json:"upgrade_family"`
	UpgradeRank          int     `json:"upgrade_rank"`
}

type SubscriptionUpgradeService struct {
	entClient       *dbent.Client
	subscriptionSvc *SubscriptionService
	configService   *PaymentConfigService
	userRepo        UserRepository
}

func NewSubscriptionUpgradeService(entClient *dbent.Client, subscriptionSvc *SubscriptionService, configService *PaymentConfigService, userRepo UserRepository) *SubscriptionUpgradeService {
	return &SubscriptionUpgradeService{
		entClient:       entClient,
		subscriptionSvc: subscriptionSvc,
		configService:   configService,
		userRepo:        userRepo,
	}
}

func (s *SubscriptionUpgradeService) ListUpgradeOptions(ctx context.Context, userID, sourceSubscriptionID int64) (*UpgradeOptionsResult, error) {
	now := time.Now()
	user, sourceSub, sourcePlan, cfg, remainingRatio, creditCNY, err := s.loadUpgradeContext(ctx, userID, sourceSubscriptionID, now)
	if err != nil {
		return nil, err
	}

	plans, err := s.configService.ListPlansForSale(ctx)
	if err != nil {
		return nil, fmt.Errorf("list upgrade plans: %w", err)
	}

	activeSubs, err := s.subscriptionSvc.ListActiveUserSubscriptions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}
	activeTargetGroups := make(map[int64]struct{}, len(activeSubs))
	for i := range activeSubs {
		if activeSubs[i].ID == sourceSub.ID {
			continue
		}
		activeTargetGroups[activeSubs[i].GroupID] = struct{}{}
	}

	options := make([]UpgradeOption, 0)
	for _, plan := range plans {
		if !s.isEligibleUpgradeTarget(plan, sourcePlan, sourceSub, activeTargetGroups) {
			continue
		}
		payable := roundCNY(math.Max(plan.Price-creditCNY, 0))
		options = append(options, UpgradeOption{
			TargetPlanID:       plan.ID,
			TargetGroupID:      plan.GroupID,
			TargetPlanName:     plan.Name,
			TargetPriceCNY:     plan.Price,
			DefaultPaymentType: preferredUpgradePaymentType(user.Balance, payable, cfg.EnabledTypes),
			PayableCNY:         payable,
			UpgradeFamily:      plan.UpgradeFamily,
			UpgradeRank:        plan.UpgradeRank,
		})
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].UpgradeRank != options[j].UpgradeRank {
			return options[i].UpgradeRank < options[j].UpgradeRank
		}
		if options[i].TargetPriceCNY != options[j].TargetPriceCNY {
			return options[i].TargetPriceCNY < options[j].TargetPriceCNY
		}
		return options[i].TargetPlanID < options[j].TargetPlanID
	})

	return &UpgradeOptionsResult{
		SourceSubscriptionID: sourceSub.ID,
		SourceGroupID:        sourceSub.GroupID,
		SourcePlanID:         *sourceSub.CurrentPlanID,
		SourcePlanName:       sourceSub.CurrentPlanName,
		RemainingRatio:       remainingRatio,
		CreditCNY:            creditCNY,
		Options:              options,
	}, nil
}

func (s *SubscriptionUpgradeService) BuildUpgradeQuote(ctx context.Context, userID, sourceSubscriptionID, targetPlanID int64, now time.Time) (*UpgradeQuote, error) {
	user, sourceSub, sourcePlan, cfg, remainingRatio, creditCNY, err := s.loadUpgradeContext(ctx, userID, sourceSubscriptionID, now)
	if err != nil {
		return nil, err
	}
	targetPlan, err := s.configService.GetPlan(ctx, targetPlanID)
	if err != nil {
		return nil, infraerrors.NotFound("UPGRADE_TARGET_PLAN_NOT_FOUND", "target subscription plan not found")
	}
	activeTargetGroups := map[int64]struct{}{}
	if targetPlan.GroupID != sourceSub.GroupID {
		if _, err := s.subscriptionSvc.GetActiveSubscription(ctx, userID, targetPlan.GroupID); err == nil {
			activeTargetGroups[targetPlan.GroupID] = struct{}{}
		}
	}
	if !s.isEligibleUpgradeTarget(targetPlan, sourcePlan, sourceSub, activeTargetGroups) {
		return nil, infraerrors.BadRequest("UPGRADE_TARGET_NOT_ALLOWED", "target subscription plan is not eligible for upgrade")
	}

	payable := roundCNY(math.Max(targetPlan.Price-creditCNY, 0))
	return &UpgradeQuote{
		SourceSubscriptionID: sourceSub.ID,
		SourceGroupID:        sourceSub.GroupID,
		SourcePlanID:         *sourceSub.CurrentPlanID,
		SourcePlanName:       sourceSub.CurrentPlanName,
		TargetPlanID:         targetPlan.ID,
		TargetGroupID:        targetPlan.GroupID,
		TargetPlanName:       targetPlan.Name,
		TargetPriceCNY:       targetPlan.Price,
		RemainingRatio:       remainingRatio,
		CreditCNY:            creditCNY,
		PayableCNY:           payable,
		DefaultPaymentType:   preferredUpgradePaymentType(user.Balance, payable, cfg.EnabledTypes),
		UpgradeFamily:        targetPlan.UpgradeFamily,
		UpgradeRank:          targetPlan.UpgradeRank,
	}, nil
}

func (s *SubscriptionUpgradeService) loadUpgradeContext(ctx context.Context, userID, sourceSubscriptionID int64, now time.Time) (*User, *UserSubscription, *dbent.SubscriptionPlan, *PaymentConfig, float64, float64, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, nil, nil, 0, 0, fmt.Errorf("get user: %w", err)
	}
	sourceSub, err := s.subscriptionSvc.GetByID(ctx, sourceSubscriptionID)
	if err != nil {
		return nil, nil, nil, nil, 0, 0, infraerrors.NotFound("UPGRADE_SOURCE_NOT_FOUND", "source subscription not found")
	}
	if sourceSub.UserID != userID {
		return nil, nil, nil, nil, 0, 0, infraerrors.Forbidden("FORBIDDEN", "no permission to upgrade this subscription")
	}
	if sourceSub.Status != SubscriptionStatusActive || !sourceSub.ExpiresAt.After(now) {
		return nil, nil, nil, nil, 0, 0, infraerrors.BadRequest("UPGRADE_SOURCE_INACTIVE", "only active subscriptions can be upgraded")
	}
	if sourceSub.CurrentPlanID == nil || sourceSub.CurrentPlanPriceCNY == nil || sourceSub.CurrentPlanValidityDays == nil || sourceSub.BillingCycleStartedAt == nil || sourceSub.CurrentPlanValidityUnit == "" {
		return nil, nil, nil, nil, 0, 0, infraerrors.BadRequest("UPGRADE_SOURCE_SNAPSHOT_MISSING", "subscription upgrade is not available for historical subscriptions without a plan snapshot")
	}
	sourcePlan, err := s.configService.GetPlan(ctx, *sourceSub.CurrentPlanID)
	if err != nil {
		return nil, nil, nil, nil, 0, 0, infraerrors.BadRequest("UPGRADE_SOURCE_PLAN_NOT_FOUND", "source subscription plan is no longer available for upgrades")
	}
	if sourcePlan.UpgradeFamily == "" {
		return nil, nil, nil, nil, 0, 0, infraerrors.BadRequest("UPGRADE_SOURCE_NOT_SUPPORTED", "subscription upgrade is not enabled for this plan")
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, nil, nil, nil, 0, 0, fmt.Errorf("get payment config: %w", err)
	}

	cycleDays := psComputeValidityDays(*sourceSub.CurrentPlanValidityDays, sourceSub.CurrentPlanValidityUnit)
	if cycleDays <= 0 {
		return nil, nil, nil, nil, 0, 0, infraerrors.BadRequest("UPGRADE_SOURCE_CYCLE_INVALID", "source subscription billing cycle is invalid")
	}
	cycleStart := *sourceSub.BillingCycleStartedAt
	cycleEnd := cycleStart.AddDate(0, 0, cycleDays)
	cycleDuration := cycleEnd.Sub(cycleStart)
	if cycleDuration <= 0 {
		return nil, nil, nil, nil, 0, 0, infraerrors.BadRequest("UPGRADE_SOURCE_CYCLE_INVALID", "source subscription billing cycle is invalid")
	}
	remainingRatio := clampRatio(sourceSub.ExpiresAt.Sub(now).Seconds() / cycleDuration.Seconds())
	creditCNY := roundCNY(*sourceSub.CurrentPlanPriceCNY * remainingRatio)

	return user, sourceSub, sourcePlan, cfg, remainingRatio, creditCNY, nil
}

func (s *SubscriptionUpgradeService) isEligibleUpgradeTarget(plan, sourcePlan *dbent.SubscriptionPlan, sourceSub *UserSubscription, activeTargetGroups map[int64]struct{}) bool {
	if plan == nil || sourcePlan == nil || sourceSub == nil {
		return false
	}
	if !plan.ForSale {
		return false
	}
	if plan.GroupID == sourceSub.GroupID {
		return false
	}
	if plan.UpgradeFamily == "" || plan.UpgradeFamily != sourcePlan.UpgradeFamily {
		return false
	}
	if plan.UpgradeRank <= sourcePlan.UpgradeRank {
		return false
	}
	if _, exists := activeTargetGroups[plan.GroupID]; exists {
		return false
	}
	return true
}

func preferredUpgradePaymentType(balance, payable float64, enabled []string) string {
	if payable <= 0 || balance >= payable {
		return payment.TypeBalance
	}
	for _, paymentType := range enabled {
		if paymentType == "" || paymentType == payment.TypeBalance {
			continue
		}
		return paymentType
	}
	return payment.TypeBalance
}

func clampRatio(value float64) float64 {
	switch {
	case math.IsNaN(value), math.IsInf(value, 0), value <= 0:
		return 0
	case value >= 1:
		return 1
	default:
		return roundRatio(value)
	}
}

func roundCNY(value float64) float64 {
	return math.Round(value*100) / 100
}

func roundRatio(value float64) float64 {
	return math.Round(value*10000) / 10000
}
