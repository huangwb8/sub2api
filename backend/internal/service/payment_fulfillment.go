package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrOrderNotFound 表示支付回调引用的订单不存在。
// Webhook handler 应将其视为不可重试终态并返回 2xx，避免支付平台重试风暴。
var ErrOrderNotFound = errors.New("payment order not found")

// --- Payment Notification & Fulfillment ---

func (s *PaymentService) HandlePaymentNotification(ctx context.Context, n *payment.PaymentNotification, pk string) error {
	if n.Status != payment.NotificationStatusSuccess {
		return nil
	}
	// Look up order by out_trade_no (the external order ID we sent to the provider)
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.OutTradeNo(n.OrderID)).Only(ctx)
	if err != nil {
		// Fallback only for true legacy "sub2_N" DB-ID payloads when the
		// current out_trade_no lookup genuinely did not find an order.
		if !dbent.IsNotFound(err) {
			return fmt.Errorf("lookup order failed for out_trade_no %s: %w", n.OrderID, err)
		}
		trimmed := strings.TrimPrefix(n.OrderID, orderIDPrefix)
		if trimmed != "" && trimmed != n.OrderID {
			if oid, parseErr := strconv.ParseInt(trimmed, 10, 64); parseErr == nil && oid > 0 {
				return s.confirmPayment(ctx, oid, n.TradeNo, n.Amount, pk, n.Metadata)
			}
		}
		return fmt.Errorf("%w: out_trade_no=%s", ErrOrderNotFound, n.OrderID)
	}
	return s.confirmPayment(ctx, order.ID, n.TradeNo, n.Amount, pk, n.Metadata)
}

func (s *PaymentService) confirmPayment(ctx context.Context, oid int64, tradeNo string, paid float64, pk string, metadata map[string]string) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		slog.Error("order not found", "orderID", oid)
		return nil
	}
	instanceProviderKey := ""
	if inst, instErr := s.getOrderProviderInstance(ctx, o); instErr == nil && inst != nil {
		instanceProviderKey = inst.ProviderKey
	}
	expectedProviderKey := expectedNotificationProviderKeyForOrder(s.registry, o, instanceProviderKey)
	if expectedProviderKey != "" && strings.TrimSpace(pk) != "" && !strings.EqualFold(expectedProviderKey, strings.TrimSpace(pk)) {
		s.writeAuditLog(ctx, o.ID, "PAYMENT_PROVIDER_MISMATCH", pk, map[string]any{
			"expectedProvider": expectedProviderKey,
			"actualProvider":   pk,
			"tradeNo":          tradeNo,
		})
		return fmt.Errorf("provider mismatch: expected %s, got %s", expectedProviderKey, pk)
	}
	if err := validateProviderNotificationMetadata(o, expectedProviderKey, metadata); err != nil {
		s.writeAuditLog(ctx, o.ID, "PAYMENT_PROVIDER_METADATA_MISMATCH", pk, map[string]any{
			"detail":   err.Error(),
			"tradeNo":  tradeNo,
			"metadata": metadata,
		})
		return err
	}
	// Skip amount check when paid=0 (e.g. QueryOrder doesn't return amount).
	// Also skip if paid is NaN/Inf (malformed provider data).
	if paid > 0 && !math.IsNaN(paid) && !math.IsInf(paid, 0) {
		if math.Abs(paid-o.PayAmount) > amountToleranceCNY {
			s.writeAuditLog(ctx, o.ID, "PAYMENT_AMOUNT_MISMATCH", pk, map[string]any{"expected": o.PayAmount, "paid": paid, "tradeNo": tradeNo})
			return fmt.Errorf("amount mismatch: expected %.2f, got %.2f", o.PayAmount, paid)
		}
	}
	// Use order's expected amount when provider didn't report one
	if paid <= 0 || math.IsNaN(paid) || math.IsInf(paid, 0) {
		paid = o.PayAmount
	}
	return s.toPaid(ctx, o, tradeNo, paid, pk)
}

func expectedNotificationProviderKeyForOrder(registry *payment.Registry, order *dbent.PaymentOrder, instanceProviderKey string) string {
	if order == nil {
		return strings.TrimSpace(instanceProviderKey)
	}
	orderProviderKey := strings.TrimSpace(psStringValue(order.ProviderKey))
	if snapshotProviderKey := providerSnapshotString(order, "provider_key"); snapshotProviderKey != "" {
		orderProviderKey = snapshotProviderKey
	}
	return expectedNotificationProviderKey(registry, order.PaymentType, orderProviderKey, instanceProviderKey)
}

func expectedNotificationProviderKey(registry *payment.Registry, orderPaymentType string, orderProviderKey string, instanceProviderKey string) string {
	if key := strings.TrimSpace(instanceProviderKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(orderProviderKey); key != "" {
		return key
	}
	if registry != nil {
		if key := strings.TrimSpace(registry.GetProviderKey(payment.PaymentType(orderPaymentType))); key != "" {
			return key
		}
	}
	return strings.TrimSpace(orderPaymentType)
}

func providerSnapshotString(order *dbent.PaymentOrder, key string) string {
	if order == nil || len(order.ProviderSnapshot) == 0 {
		return ""
	}
	value, ok := order.ProviderSnapshot[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func validateProviderNotificationMetadata(order *dbent.PaymentOrder, providerKey string, metadata map[string]string) error {
	if order == nil || len(metadata) == 0 || !strings.EqualFold(strings.TrimSpace(providerKey), payment.TypeWxpay) {
		return nil
	}

	if expected := providerSnapshotString(order, "merchant_app_id"); expected != "" {
		actual := strings.TrimSpace(metadata["appid"])
		if actual == "" {
			return fmt.Errorf("wxpay notification missing appid")
		}
		if !strings.EqualFold(expected, actual) {
			return fmt.Errorf("wxpay appid mismatch: expected %s, got %s", expected, actual)
		}
	}
	if expected := providerSnapshotString(order, "merchant_id"); expected != "" {
		actual := strings.TrimSpace(metadata["mchid"])
		if actual == "" {
			return fmt.Errorf("wxpay notification missing mchid")
		}
		if !strings.EqualFold(expected, actual) {
			return fmt.Errorf("wxpay mchid mismatch: expected %s, got %s", expected, actual)
		}
	}
	if expected := strings.ToUpper(providerSnapshotString(order, "currency")); expected != "" {
		actual := strings.ToUpper(strings.TrimSpace(metadata["currency"]))
		if actual == "" {
			return fmt.Errorf("wxpay notification missing currency")
		}
		if !strings.EqualFold(expected, actual) {
			return fmt.Errorf("wxpay currency mismatch: expected %s, got %s", expected, actual)
		}
	}
	if actual := strings.TrimSpace(metadata["trade_state"]); actual != "" && !strings.EqualFold(actual, "SUCCESS") {
		return fmt.Errorf("wxpay trade_state mismatch: expected SUCCESS, got %s", actual)
	}

	return nil
}

func psStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func (s *PaymentService) toPaid(ctx context.Context, o *dbent.PaymentOrder, tradeNo string, paid float64, pk string) error {
	previousStatus := o.Status
	now := time.Now()
	grace := now.Add(-paymentGraceMinutes * time.Minute)
	c, err := s.entClient.PaymentOrder.Update().Where(
		paymentorder.IDEQ(o.ID),
		paymentorder.Or(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.StatusEQ(OrderStatusCancelled),
			paymentorder.And(
				paymentorder.StatusEQ(OrderStatusExpired),
				paymentorder.UpdatedAtGTE(grace),
			),
		),
	).SetStatus(OrderStatusPaid).SetPayAmount(paid).SetPaymentTradeNo(tradeNo).SetPaidAt(now).ClearFailedAt().ClearFailedReason().Save(ctx)
	if err != nil {
		return fmt.Errorf("update to PAID: %w", err)
	}
	if c == 0 {
		return s.alreadyProcessed(ctx, o)
	}
	if previousStatus == OrderStatusCancelled || previousStatus == OrderStatusExpired {
		slog.Info("order recovered from webhook payment success",
			"orderID", o.ID,
			"previousStatus", previousStatus,
			"tradeNo", tradeNo,
			"provider", pk,
		)
		s.writeAuditLog(ctx, o.ID, "ORDER_RECOVERED", pk, map[string]any{
			"previous_status": previousStatus,
			"tradeNo":         tradeNo,
			"paidAmount":      paid,
			"reason":          "webhook payment success received after order " + previousStatus,
		})
	}
	s.writeAuditLog(ctx, o.ID, "ORDER_PAID", pk, map[string]any{"tradeNo": tradeNo, "paidAmount": paid})
	return s.executeFulfillment(ctx, o.ID)
}

func (s *PaymentService) alreadyProcessed(ctx context.Context, o *dbent.PaymentOrder) error {
	cur, err := s.entClient.PaymentOrder.Get(ctx, o.ID)
	if err != nil {
		return nil
	}
	switch cur.Status {
	case OrderStatusCompleted, OrderStatusRefunded:
		return nil
	case OrderStatusFailed:
		return s.executeFulfillment(ctx, o.ID)
	case OrderStatusPaid, OrderStatusRecharging:
		return fmt.Errorf("order %d is being processed", o.ID)
	case OrderStatusExpired:
		slog.Warn("webhook payment success for expired order beyond grace period",
			"orderID", o.ID,
			"status", cur.Status,
			"updatedAt", cur.UpdatedAt,
		)
		s.writeAuditLog(ctx, o.ID, "PAYMENT_AFTER_EXPIRY", "system", map[string]any{
			"status":    cur.Status,
			"updatedAt": cur.UpdatedAt,
			"reason":    "payment arrived after expiry grace period",
		})
		return nil
	default:
		return nil
	}
}

func (s *PaymentService) executeFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	if o.OrderType == payment.OrderTypeSubscriptionUpgrade {
		return s.ExecuteSubscriptionUpgradeFulfillment(ctx, oid)
	}
	if o.OrderType == payment.OrderTypeSubscription {
		return s.ExecuteSubscriptionFulfillment(ctx, oid)
	}
	return s.ExecuteBalanceFulfillment(ctx, oid)
}

func (s *PaymentService) ExecuteBalanceFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed)).SetStatus(OrderStatusRecharging).Save(ctx)
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil
	}
	if err := s.doBalance(ctx, o); err != nil {
		s.markFailed(ctx, oid, err)
		return err
	}
	return nil
}

// redeemAction represents the idempotency decision for balance fulfillment.
type redeemAction int

const (
	// redeemActionCreate: code does not exist — create it, then redeem.
	redeemActionCreate redeemAction = iota
	// redeemActionRedeem: code exists but is unused — skip creation, redeem only.
	redeemActionRedeem
	// redeemActionSkipCompleted: code exists and is already used — skip to mark completed.
	redeemActionSkipCompleted
)

// resolveRedeemAction decides the idempotency action based on an existing redeem code lookup.
// existing is the result of GetByCode; lookupErr is the error from that call.
func resolveRedeemAction(existing *RedeemCode, lookupErr error) redeemAction {
	if existing == nil || lookupErr != nil {
		return redeemActionCreate
	}
	if existing.IsUsed() {
		return redeemActionSkipCompleted
	}
	return redeemActionRedeem
}

func (s *PaymentService) doBalance(ctx context.Context, o *dbent.PaymentOrder) error {
	// Idempotency: check if redeem code already exists (from a previous partial run)
	existing, lookupErr := s.redeemService.GetByCode(ctx, o.RechargeCode)
	action := resolveRedeemAction(existing, lookupErr)

	switch action {
	case redeemActionSkipCompleted:
		if err := s.applyAffiliateRebateForOrder(ctx, o); err != nil {
			return err
		}
		// Code already created and redeemed — just mark completed
		return s.markCompleted(ctx, o, "RECHARGE_SUCCESS")
	case redeemActionCreate:
		rc := &RedeemCode{Code: o.RechargeCode, Type: RedeemTypeBalance, Value: o.Amount, Status: StatusUnused}
		if err := s.redeemService.CreateCode(ctx, rc); err != nil {
			return fmt.Errorf("create redeem code: %w", err)
		}
	case redeemActionRedeem:
		// Code exists but unused — skip creation, proceed to redeem
	}
	if _, err := s.redeemService.Redeem(ctx, o.UserID, o.RechargeCode); err != nil {
		return fmt.Errorf("redeem balance: %w", err)
	}
	if err := s.applyAffiliateRebateForOrder(ctx, o); err != nil {
		return err
	}
	return s.markCompleted(ctx, o, "RECHARGE_SUCCESS")
}

func (s *PaymentService) applyAffiliateRebateForOrder(ctx context.Context, o *dbent.PaymentOrder) error {
	if s == nil || s.affiliateSvc == nil || o == nil || o.OrderType != payment.OrderTypeBalance {
		return nil
	}
	if s.hasAuditLog(ctx, o.ID, "AFFILIATE_REBATE_APPLIED") || s.hasAuditLog(ctx, o.ID, "AFFILIATE_REBATE_SKIPPED") {
		return nil
	}
	if !s.affiliateSvc.IsEnabled(ctx) {
		s.writeAuditLog(ctx, o.ID, "AFFILIATE_REBATE_SKIPPED", "system", map[string]any{
			"reason": "affiliate disabled",
		})
		return nil
	}
	rebate, err := s.affiliateSvc.AccrueInviteRebate(ctx, o.UserID, o.Amount)
	if err != nil {
		return fmt.Errorf("affiliate rebate: %w", err)
	}
	if rebate <= 0 {
		s.writeAuditLog(ctx, o.ID, "AFFILIATE_REBATE_SKIPPED", "system", map[string]any{
			"reason": "no eligible inviter",
		})
		return nil
	}
	s.writeAuditLog(ctx, o.ID, "AFFILIATE_REBATE_APPLIED", "system", map[string]any{
		"rebate": rebate,
	})
	return nil
}

func (s *PaymentService) markCompleted(ctx context.Context, o *dbent.PaymentOrder, auditAction string) error {
	now := time.Now()
	_, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(o.ID), paymentorder.StatusEQ(OrderStatusRecharging)).SetStatus(OrderStatusCompleted).SetCompletedAt(now).Save(ctx)
	if err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	s.writeAuditLog(ctx, o.ID, auditAction, "system", map[string]any{"rechargeCode": o.RechargeCode, "amount": o.Amount})
	return nil
}

func (s *PaymentService) ExecuteSubscriptionFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	if o.SubscriptionGroupID == nil || o.SubscriptionDays == nil {
		return infraerrors.BadRequest("INVALID_STATUS", "missing subscription info")
	}
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed)).SetStatus(OrderStatusRecharging).Save(ctx)
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil
	}
	if err := s.doSub(ctx, o); err != nil {
		s.markFailed(ctx, oid, err)
		return err
	}
	return nil
}

func (s *PaymentService) ExecuteSubscriptionUpgradeFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	if o.PlanID == nil || o.SourceSubscriptionID == nil {
		return infraerrors.BadRequest("INVALID_STATUS", "missing upgrade order info")
	}
	c, err := s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(oid), paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed)).
		SetStatus(OrderStatusRecharging).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil
	}
	if err := s.doSubscriptionUpgrade(ctx, o); err != nil {
		s.markFailed(ctx, oid, err)
		return err
	}
	return nil
}

func (s *PaymentService) doSub(ctx context.Context, o *dbent.PaymentOrder) error {
	gid := *o.SubscriptionGroupID
	days := *o.SubscriptionDays
	var plan *dbent.SubscriptionPlan
	g, err := s.groupRepo.GetByID(ctx, gid)
	if err != nil || g.Status != payment.EntityStatusActive {
		return fmt.Errorf("group %d no longer exists or inactive", gid)
	}
	if o.PlanID != nil {
		plan, err = s.configService.GetPlan(ctx, *o.PlanID)
		if err != nil {
			return fmt.Errorf("load subscription plan: %w", err)
		}
	}
	// Idempotency: check audit log to see if subscription was already assigned.
	// Prevents double-extension on retry after markCompleted fails.
	if s.hasAuditLog(ctx, o.ID, "SUBSCRIPTION_SUCCESS") {
		slog.Info("subscription already assigned for order, skipping", "orderID", o.ID, "groupID", gid)
		return s.markCompleted(ctx, o, "SUBSCRIPTION_SUCCESS")
	}
	orderNote := fmt.Sprintf("payment order %d", o.ID)
	_, _, err = s.subscriptionSvc.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
		UserID:       o.UserID,
		GroupID:      gid,
		ValidityDays: days,
		AssignedBy:   0,
		Notes:        orderNote,
		PlanSnapshot: subscriptionPlanSnapshotFromPlan(plan),
	})
	if err != nil {
		return fmt.Errorf("assign subscription: %w", err)
	}
	return s.markCompleted(ctx, o, "SUBSCRIPTION_SUCCESS")
}

func (s *PaymentService) doSubscriptionUpgrade(ctx context.Context, o *dbent.PaymentOrder) error {
	targetPlan, err := s.configService.GetPlan(ctx, *o.PlanID)
	if err != nil {
		return fmt.Errorf("load target plan: %w", err)
	}
	now := time.Now()
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	sourceSub, err := tx.UserSubscription.Get(txCtx, *o.SourceSubscriptionID)
	if err != nil {
		return fmt.Errorf("load source subscription: %w", err)
	}
	if sourceSub.UserID != o.UserID {
		return infraerrors.Forbidden("FORBIDDEN", "source subscription does not belong to the order owner")
	}
	if sourceSub.Status != SubscriptionStatusActive || !sourceSub.ExpiresAt.After(now) {
		return infraerrors.BadRequest("UPGRADE_SOURCE_INACTIVE", "source subscription is no longer active")
	}
	if o.SourcePlanID != nil && sourceSub.CurrentPlanID != nil && *o.SourcePlanID != *sourceSub.CurrentPlanID {
		return infraerrors.Conflict("UPGRADE_SOURCE_CHANGED", "source subscription plan has changed since the upgrade order was created")
	}

	revokeNotes := ""
	if sourceSub.Notes != nil {
		revokeNotes = *sourceSub.Notes
	}
	if revokeNotes != "" {
		revokeNotes += "\n"
	}
	revokeNotes += fmt.Sprintf("revoked by subscription upgrade order %d -> plan %d", o.ID, targetPlan.ID)
	if _, err := tx.UserSubscription.UpdateOneID(sourceSub.ID).
		SetStatus(SubscriptionStatusExpired).
		SetExpiresAt(now).
		SetNotes(revokeNotes).
		Save(txCtx); err != nil {
		return fmt.Errorf("revoke source subscription: %w", err)
	}

	if _, err := s.subscriptionSvc.AssignSubscription(txCtx, &AssignSubscriptionInput{
		UserID:                o.UserID,
		GroupID:               targetPlan.GroupID,
		ValidityDays:          psComputeValidityDays(targetPlan.ValidityDays, targetPlan.ValidityUnit),
		AssignedBy:            0,
		Notes:                 fmt.Sprintf("upgraded from subscription %d via payment order %d", sourceSub.ID, o.ID),
		PlanSnapshot:          subscriptionPlanSnapshotFromPlan(targetPlan),
		BillingCycleStartedAt: &now,
	}); err != nil {
		return fmt.Errorf("create target subscription: %w", err)
	}

	if _, err := tx.PaymentOrder.UpdateOneID(o.ID).
		SetStatus(OrderStatusCompleted).
		SetCompletedAt(now).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx); err != nil {
		return fmt.Errorf("mark upgrade order completed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upgrade fulfillment: %w", err)
	}

	s.subscriptionSvc.InvalidateSubCache(o.UserID, sourceSub.GroupID)
	s.subscriptionSvc.InvalidateSubCache(o.UserID, targetPlan.GroupID)
	s.writeAuditLog(ctx, o.ID, "UPGRADE_SUCCESS", "system", map[string]any{
		"sourceSubscriptionID": sourceSub.ID,
		"targetPlanID":         targetPlan.ID,
		"targetGroupID":        targetPlan.GroupID,
	})
	return nil
}

func (s *PaymentService) hasAuditLog(ctx context.Context, orderID int64, action string) bool {
	oid := strconv.FormatInt(orderID, 10)
	c, _ := s.entClient.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(oid), paymentauditlog.ActionEQ(action)).
		Limit(1).Count(ctx)
	return c > 0
}

func (s *PaymentService) markFailed(ctx context.Context, oid int64, cause error) {
	now := time.Now()
	r := psErrMsg(cause)
	// Only mark FAILED if still in RECHARGING state — prevents overwriting
	// a COMPLETED order when markCompleted failed but fulfillment succeeded.
	c, e := s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(oid), paymentorder.StatusEQ(OrderStatusRecharging)).
		SetStatus(OrderStatusFailed).SetFailedAt(now).SetFailedReason(r).Save(ctx)
	if e != nil {
		slog.Error("mark FAILED", "orderID", oid, "error", e)
	}
	if c > 0 {
		s.writeAuditLog(ctx, oid, "FULFILLMENT_FAILED", "system", map[string]any{"reason": r})
	}
}

func (s *PaymentService) RetryFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.PaidAt == nil {
		return infraerrors.BadRequest("INVALID_STATUS", "order is not paid")
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot retry")
	}
	if o.Status == OrderStatusRecharging {
		return infraerrors.Conflict("CONFLICT", "order is being processed")
	}
	if o.Status == OrderStatusCompleted {
		return infraerrors.BadRequest("INVALID_STATUS", "order already completed")
	}
	if o.Status != OrderStatusFailed && o.Status != OrderStatusPaid {
		return infraerrors.BadRequest("INVALID_STATUS", "only paid and failed orders can retry")
	}
	_, err = s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.StatusIn(OrderStatusFailed, OrderStatusPaid)).SetStatus(OrderStatusPaid).ClearFailedAt().ClearFailedReason().Save(ctx)
	if err != nil {
		return fmt.Errorf("reset for retry: %w", err)
	}
	s.writeAuditLog(ctx, oid, "RECHARGE_RETRY", "admin", map[string]any{"detail": "admin manual retry"})
	return s.executeFulfillment(ctx, oid)
}
