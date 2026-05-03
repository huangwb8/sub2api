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
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Order Creation ---

func (s *PaymentService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	if req.OrderType == "" {
		req.OrderType = payment.OrderTypeBalance
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}
	plan, err := s.validateOrderInput(ctx, req, cfg)
	if err != nil {
		return nil, err
	}
	if err := s.checkCancelRateLimit(ctx, req.UserID, cfg); err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	if req.OrderType == payment.OrderTypeSubscriptionUpgrade {
		return s.createSubscriptionUpgradeOrder(ctx, req, user, plan, cfg)
	}
	amount := req.Amount
	if plan != nil {
		amount = plan.Price
	}
	if req.PaymentType == payment.TypeBalance {
		return s.createBalanceSubscriptionOrder(ctx, req, user, plan, cfg, amount)
	}
	feeRate := s.getFeeRate(req.PaymentType)
	payAmountStr := payment.CalculatePayAmount(amount, feeRate)
	payAmount, _ := strconv.ParseFloat(payAmountStr, 64)
	sel, err := s.selectPaymentInstance(ctx, req.PaymentType, cfg, payAmount)
	if err != nil {
		return nil, err
	}
	order, err := s.createOrderInTx(ctx, req, user, plan, cfg, amount, feeRate, payAmount, sel)
	if err != nil {
		return nil, err
	}
	resp, err := s.invokeProvider(ctx, order, req, cfg, payAmountStr, payAmount, plan, sel)
	if err != nil {
		_, _ = s.entClient.PaymentOrder.UpdateOneID(order.ID).
			SetStatus(OrderStatusFailed).
			Save(ctx)
		return nil, err
	}
	return resp, nil
}

func (s *PaymentService) validateOrderInput(ctx context.Context, req CreateOrderRequest, cfg *PaymentConfig) (*dbent.SubscriptionPlan, error) {
	if req.PaymentType == payment.TypeBalance {
		if req.OrderType != payment.OrderTypeSubscription && req.OrderType != payment.OrderTypeSubscriptionUpgrade {
			return nil, infraerrors.BadRequest("INVALID_PAYMENT_TYPE", "balance payment is only available for subscription orders")
		}
		return s.validateSubOrder(ctx, req)
	}
	if req.OrderType == payment.OrderTypeBalance && cfg.BalanceDisabled {
		return nil, infraerrors.Forbidden("BALANCE_PAYMENT_DISABLED", "balance recharge has been disabled")
	}
	if req.OrderType == payment.OrderTypeSubscription {
		return s.validateSubOrder(ctx, req)
	}
	if req.OrderType == payment.OrderTypeSubscriptionUpgrade {
		return s.validateSubOrder(ctx, req)
	}
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount <= 0 {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount must be a positive number")
	}
	if (cfg.MinAmount > 0 && req.Amount < cfg.MinAmount) || (cfg.MaxAmount > 0 && req.Amount > cfg.MaxAmount) {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount out of range").
			WithMetadata(map[string]string{"min": fmt.Sprintf("%.2f", cfg.MinAmount), "max": fmt.Sprintf("%.2f", cfg.MaxAmount)})
	}
	return nil, nil
}

func (s *PaymentService) validateSubOrder(ctx context.Context, req CreateOrderRequest) (*dbent.SubscriptionPlan, error) {
	if req.PlanID == 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription order requires a plan")
	}
	plan, err := s.configService.GetPlan(ctx, req.PlanID)
	if err != nil || !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}
	group, err := s.groupRepo.GetByID(ctx, plan.GroupID)
	if err != nil || group.Status != payment.EntityStatusActive {
		return nil, infraerrors.NotFound("GROUP_NOT_FOUND", "subscription group is no longer available")
	}
	if !group.IsSubscriptionType() {
		return nil, infraerrors.BadRequest("GROUP_TYPE_MISMATCH", "group is not a subscription type")
	}
	return plan, nil
}

func (s *PaymentService) selectPaymentInstance(ctx context.Context, paymentType string, cfg *PaymentConfig, payAmount float64) (*payment.InstanceSelection, error) {
	sel, err := s.loadBalancer.SelectInstance(ctx, paymentType, cfg.EnabledTypes, payment.Strategy(cfg.LoadBalanceStrategy), payAmount)
	if err != nil {
		return nil, fmt.Errorf("select provider instance: %w", err)
	}
	if sel == nil {
		return nil, infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "no_available_instance")
	}
	return sel, nil
}

func buildPaymentOrderProviderSnapshot(sel *payment.InstanceSelection) map[string]any {
	if sel == nil {
		return nil
	}

	snapshot := map[string]any{
		"schema_version": 1,
	}
	if instanceID := strings.TrimSpace(sel.InstanceID); instanceID != "" {
		snapshot["provider_instance_id"] = instanceID
	}
	if providerKey := strings.TrimSpace(sel.ProviderKey); providerKey != "" {
		snapshot["provider_key"] = providerKey
		switch providerKey {
		case payment.TypeWxpay:
			if appID := strings.TrimSpace(sel.Config["appId"]); appID != "" {
				snapshot["merchant_app_id"] = appID
			}
			if merchantID := strings.TrimSpace(sel.Config["mchId"]); merchantID != "" {
				snapshot["merchant_id"] = merchantID
			}
			snapshot["currency"] = "CNY"
		case payment.TypeAlipay:
			if appID := strings.TrimSpace(sel.Config["appId"]); appID != "" {
				snapshot["merchant_app_id"] = appID
			}
		case payment.TypeEasyPay:
			if merchantID := strings.TrimSpace(sel.Config["pid"]); merchantID != "" {
				snapshot["merchant_id"] = merchantID
			}
		}
	}
	if paymentMode := strings.TrimSpace(sel.PaymentMode); paymentMode != "" {
		snapshot["payment_mode"] = paymentMode
	}
	if len(snapshot) == 1 {
		return nil
	}
	return snapshot
}

func (s *PaymentService) createOrderInTx(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, amount, feeRate, payAmount float64, sel *payment.InstanceSelection) (*dbent.PaymentOrder, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.checkPendingLimit(ctx, tx, req.UserID, cfg.MaxPendingOrders); err != nil {
		return nil, err
	}
	if req.OrderType == payment.OrderTypeBalance {
		if err := s.checkDailyLimit(ctx, tx, req.UserID, amount, cfg.DailyLimit); err != nil {
			return nil, err
		}
	}
	tm := cfg.OrderTimeoutMin
	if tm <= 0 {
		tm = defaultOrderTimeoutMin
	}
	exp := time.Now().Add(time.Duration(tm) * time.Minute)
	b := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetNillableUserNotes(psNilIfEmpty(user.Notes)).
		SetAmount(amount).
		SetPayAmount(payAmount).
		SetFeeRate(feeRate).
		SetRechargeCode("").
		SetOutTradeNo(generateOutTradeNo()).
		SetPaymentType(req.PaymentType).
		SetPaymentTradeNo("").
		SetOrderType(req.OrderType).
		SetStatus(OrderStatusPending).
		SetExpiresAt(exp).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost)
	if sel != nil {
		if instanceID := strings.TrimSpace(sel.InstanceID); instanceID != "" {
			b.SetProviderInstanceID(instanceID)
		}
		if providerKey := strings.TrimSpace(sel.ProviderKey); providerKey != "" {
			b.SetProviderKey(providerKey)
		}
		if snapshot := buildPaymentOrderProviderSnapshot(sel); snapshot != nil {
			b.SetProviderSnapshot(snapshot)
		}
	}
	if req.SrcURL != "" {
		b.SetSrcURL(req.SrcURL)
	}
	if plan != nil {
		b.SetPlanID(plan.ID).SetSubscriptionGroupID(plan.GroupID).SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit))
	}
	order, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	code := fmt.Sprintf("PAY-%d-%d", order.ID, time.Now().UnixNano()%100000)
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetRechargeCode(code).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set recharge code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit order transaction: %w", err)
	}
	return order, nil
}

func (s *PaymentService) checkPendingLimit(ctx context.Context, tx *dbent.Tx, userID int64, max int) error {
	if max <= 0 {
		max = defaultMaxPendingOrders
	}
	c, err := tx.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID), paymentorder.StatusEQ(OrderStatusPending)).Count(ctx)
	if err != nil {
		return fmt.Errorf("count pending orders: %w", err)
	}
	if c >= max {
		return infraerrors.TooManyRequests("TOO_MANY_PENDING", fmt.Sprintf("too many pending orders (max %d)", max)).
			WithMetadata(map[string]string{"max": strconv.Itoa(max)})
	}
	return nil
}

func (s *PaymentService) checkDailyLimit(ctx context.Context, tx *dbent.Tx, userID int64, amount, limit float64) error {
	if limit <= 0 {
		return nil
	}
	ts := psStartOfDayUTC(time.Now())
	orders, err := tx.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID), paymentorder.StatusIn(OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted), paymentorder.PaidAtGTE(ts)).All(ctx)
	if err != nil {
		return fmt.Errorf("query daily usage: %w", err)
	}
	var used float64
	for _, o := range orders {
		if o.OrderType != payment.OrderTypeBalance {
			continue
		}
		used += o.Amount
	}
	if used+amount > limit {
		return infraerrors.TooManyRequests("DAILY_LIMIT_EXCEEDED", "daily_limit_exceeded").
			WithMetadata(map[string]string{"remaining": fmt.Sprintf("%.2f", math.Max(0, limit-used))})
	}
	return nil
}

func enrichPaymentProviderError(err error, providerKey, instanceID string) error {
	var appErr *infraerrors.ApplicationError
	if !errors.As(err, &appErr) {
		return nil
	}
	metadata := map[string]string{}
	if providerKey != "" {
		metadata["provider"] = providerKey
	}
	if instanceID != "" {
		metadata["instance_id"] = instanceID
	}
	for k, v := range appErr.Metadata {
		metadata[k] = v
	}
	if len(metadata) == 0 {
		return appErr
	}
	return appErr.WithMetadata(metadata)
}

func wrapPaymentProviderInitError(err error, providerKey, instanceID string) error {
	if enriched := enrichPaymentProviderError(err, providerKey, instanceID); enriched != nil {
		return enriched
	}
	return infraerrors.ServiceUnavailable("PAYMENT_PROVIDER_MISCONFIGURED", "payment_provider_misconfigured").
		WithMetadata(map[string]string{"provider": providerKey, "instance_id": instanceID})
}

func wrapPaymentProviderRuntimeError(err error, providerKey, instanceID string) error {
	if enriched := enrichPaymentProviderError(err, providerKey, instanceID); enriched != nil {
		return enriched
	}
	return infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", "payment_gateway_error").
		WithMetadata(map[string]string{"provider": providerKey, "instance_id": instanceID})
}

func (s *PaymentService) invokeProvider(ctx context.Context, order *dbent.PaymentOrder, req CreateOrderRequest, cfg *PaymentConfig, payAmountStr string, payAmount float64, plan *dbent.SubscriptionPlan, sel *payment.InstanceSelection) (*CreateOrderResponse, error) {
	prov, err := provider.CreateProvider(sel.ProviderKey, sel.InstanceID, sel.Config)
	if err != nil {
		slog.Error("[PaymentService] CreateProvider failed", "provider", sel.ProviderKey, "instance", sel.InstanceID, "error", err)
		return nil, wrapPaymentProviderInitError(err, sel.ProviderKey, sel.InstanceID)
	}
	subject := s.buildPaymentSubject(plan, payAmountStr, cfg)
	outTradeNo := order.OutTradeNo
	pr, err := prov.CreatePayment(ctx, payment.CreatePaymentRequest{OrderID: outTradeNo, Amount: payAmountStr, PaymentType: req.PaymentType, Subject: subject, ClientIP: req.ClientIP, IsMobile: req.IsMobile, InstanceSubMethods: sel.SupportedTypes})
	if err != nil {
		slog.Error("[PaymentService] CreatePayment failed", "provider", sel.ProviderKey, "instance", sel.InstanceID, "error", err)
		return nil, wrapPaymentProviderRuntimeError(err, sel.ProviderKey, sel.InstanceID)
	}
	update := s.entClient.PaymentOrder.UpdateOneID(order.ID).
		SetNillablePaymentTradeNo(psNilIfEmpty(pr.TradeNo)).
		SetNillablePayURL(psNilIfEmpty(pr.PayURL)).
		SetNillableQrCode(psNilIfEmpty(pr.QRCode)).
		SetNillableProviderInstanceID(psNilIfEmpty(sel.InstanceID)).
		SetNillableProviderKey(psNilIfEmpty(sel.ProviderKey))
	if snapshot := buildPaymentOrderProviderSnapshot(sel); snapshot != nil {
		update.SetProviderSnapshot(snapshot)
	}
	_, err = update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update order with payment details: %w", err)
	}
	s.writeAuditLog(ctx, order.ID, "ORDER_CREATED", fmt.Sprintf("user:%d", req.UserID), map[string]any{"amount": req.Amount, "paymentType": req.PaymentType, "orderType": req.OrderType})
	return &CreateOrderResponse{
		OrderID:              order.ID,
		Amount:               order.Amount,
		PayAmount:            payAmount,
		FeeRate:              order.FeeRate,
		Status:               OrderStatusPending,
		PaymentType:          req.PaymentType,
		PayURL:               pr.PayURL,
		QRCode:               pr.QRCode,
		ClientSecret:         pr.ClientSecret,
		StripePublishableKey: selectedStripePublishableKey(sel),
		ExpiresAt:            order.ExpiresAt,
		PaymentMode:          sel.PaymentMode,
	}, nil
}

func (s *PaymentService) createBalanceSubscriptionOrder(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, amount float64) (*CreateOrderResponse, error) {
	if plan == nil {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription order requires a plan")
	}

	order, err := s.createBalanceSubscriptionOrderInTx(ctx, req, user, plan, cfg, amount)
	if err != nil {
		return nil, err
	}
	s.invalidateBalanceAfterWalletDebit(ctx, req.UserID, order.ID, amount)

	if err := s.ExecuteSubscriptionFulfillment(ctx, order.ID); err != nil {
		if rollbackErr := s.userRepo.UpdateBalance(ctx, req.UserID, amount); rollbackErr != nil {
			s.writeAuditLog(ctx, order.ID, "BALANCE_PAYMENT_ROLLBACK_FAILED", "system", map[string]any{
				"amount":        amount,
				"originalError": err.Error(),
				"rollbackError": rollbackErr.Error(),
			})
			s.markFailed(ctx, order.ID, fmt.Errorf("assign subscription: %w", err))
			s.invalidateBalanceAfterWalletDebit(ctx, req.UserID, order.ID, amount)
			return nil, fmt.Errorf("assign subscription: %w (balance rollback failed: %w)", err, rollbackErr)
		}
		s.writeAuditLog(ctx, order.ID, "BALANCE_PAYMENT_ROLLED_BACK", "system", map[string]any{
			"amount": amount,
			"reason": err.Error(),
		})
		s.markFailed(ctx, order.ID, fmt.Errorf("assign subscription: %w", err))
		s.invalidateBalanceAfterWalletDebit(ctx, req.UserID, order.ID, amount)
		return nil, fmt.Errorf("assign subscription: %w", err)
	}

	completedOrder, err := s.entClient.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("reload order: %w", err)
	}

	s.writeAuditLog(ctx, order.ID, "ORDER_PAID", "system", map[string]any{
		"paymentType": req.PaymentType,
		"paidAmount":  amount,
		"source":      "balance",
	})

	return &CreateOrderResponse{
		OrderID:     completedOrder.ID,
		Amount:      completedOrder.Amount,
		PayAmount:   completedOrder.PayAmount,
		FeeRate:     completedOrder.FeeRate,
		Status:      completedOrder.Status,
		PaymentType: completedOrder.PaymentType,
		ExpiresAt:   completedOrder.ExpiresAt,
	}, nil
}

func subscriptionPlanSnapshotFromPlan(plan *dbent.SubscriptionPlan) *SubscriptionPlanSnapshot {
	if plan == nil {
		return nil
	}
	planID := plan.ID
	price := plan.Price
	validityDays := plan.ValidityDays
	return &SubscriptionPlanSnapshot{
		PlanID:       &planID,
		PlanName:     plan.Name,
		PlanPriceCNY: &price,
		ValidityDays: &validityDays,
		ValidityUnit: plan.ValidityUnit,
	}
}

func (s *PaymentService) createSubscriptionUpgradeOrder(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, cfg *PaymentConfig) (*CreateOrderResponse, error) {
	if plan == nil {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription upgrade order requires a target plan")
	}
	if req.SourceSubscriptionID == 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription upgrade order requires a source subscription")
	}

	upgradeSvc := NewSubscriptionUpgradeService(s.entClient, s.subscriptionSvc, s.configService, s.userRepo)
	quote, err := upgradeSvc.BuildUpgradeQuote(ctx, req.UserID, req.SourceSubscriptionID, plan.ID, time.Now())
	if err != nil {
		return nil, err
	}

	if err := s.ensureNoPendingUpgradeOrder(ctx, req.SourceSubscriptionID); err != nil {
		return nil, err
	}

	amount := quote.PayableCNY
	if amount <= 0 {
		order, err := s.createZeroSubscriptionUpgradeOrder(ctx, req, user, plan, quote, cfg)
		if err != nil {
			return nil, err
		}
		if err := s.ExecuteSubscriptionUpgradeFulfillment(ctx, order.ID); err != nil {
			return nil, err
		}
		completedOrder, err := s.entClient.PaymentOrder.Get(ctx, order.ID)
		if err != nil {
			return nil, fmt.Errorf("reload order: %w", err)
		}
		return &CreateOrderResponse{
			OrderID:     completedOrder.ID,
			Amount:      completedOrder.Amount,
			PayAmount:   completedOrder.PayAmount,
			FeeRate:     completedOrder.FeeRate,
			Status:      completedOrder.Status,
			PaymentType: completedOrder.PaymentType,
			ExpiresAt:   completedOrder.ExpiresAt,
		}, nil
	}

	if req.PaymentType == payment.TypeBalance {
		order, err := s.createBalanceSubscriptionUpgradeOrderInTx(ctx, req, user, plan, quote, cfg, amount)
		if err != nil {
			return nil, err
		}
		s.invalidateBalanceAfterWalletDebit(ctx, req.UserID, order.ID, amount)
		if err := s.ExecuteSubscriptionUpgradeFulfillment(ctx, order.ID); err != nil {
			if rollbackErr := s.userRepo.UpdateBalance(ctx, req.UserID, amount); rollbackErr != nil {
				s.invalidateBalanceAfterWalletDebit(ctx, req.UserID, order.ID, amount)
				return nil, fmt.Errorf("upgrade fulfillment failed: %w (balance rollback failed: %w)", err, rollbackErr)
			}
			s.invalidateBalanceAfterWalletDebit(ctx, req.UserID, order.ID, amount)
			return nil, err
		}
		completedOrder, err := s.entClient.PaymentOrder.Get(ctx, order.ID)
		if err != nil {
			return nil, fmt.Errorf("reload order: %w", err)
		}
		return &CreateOrderResponse{
			OrderID:     completedOrder.ID,
			Amount:      completedOrder.Amount,
			PayAmount:   completedOrder.PayAmount,
			FeeRate:     completedOrder.FeeRate,
			Status:      completedOrder.Status,
			PaymentType: completedOrder.PaymentType,
			ExpiresAt:   completedOrder.ExpiresAt,
		}, nil
	}

	feeRate := s.getFeeRate(req.PaymentType)
	payAmountStr := payment.CalculatePayAmount(amount, feeRate)
	payAmount, _ := strconv.ParseFloat(payAmountStr, 64)
	sel, err := s.selectPaymentInstance(ctx, req.PaymentType, cfg, payAmount)
	if err != nil {
		return nil, err
	}
	order, err := s.createSubscriptionUpgradeOrderInTx(ctx, req, user, plan, quote, cfg, amount, feeRate, payAmount, OrderStatusPending, nil, sel)
	if err != nil {
		return nil, err
	}
	resp, err := s.invokeProvider(ctx, order, req, cfg, payAmountStr, payAmount, plan, sel)
	if err != nil {
		_, _ = s.entClient.PaymentOrder.UpdateOneID(order.ID).
			SetStatus(OrderStatusFailed).
			Save(ctx)
		return nil, err
	}
	return resp, nil
}

func (s *PaymentService) ensureNoPendingUpgradeOrder(ctx context.Context, sourceSubscriptionID int64) error {
	exists, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscriptionUpgrade),
			paymentorder.SourceSubscriptionID(sourceSubscriptionID),
			paymentorder.Or(
				paymentorder.StatusIn(OrderStatusPending, OrderStatusPaid, OrderStatusRecharging),
				paymentorder.And(
					paymentorder.StatusEQ(OrderStatusFailed),
					paymentorder.PaidAtNotNil(),
				),
			),
		).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("check pending upgrade orders: %w", err)
	}
	if exists {
		return infraerrors.Conflict("UPGRADE_ORDER_EXISTS", "there is already an unfinished upgrade order for this subscription")
	}
	return nil
}

func (s *PaymentService) invalidateBalanceAfterWalletDebit(ctx context.Context, userID, orderID int64, amount float64) {
	if s == nil || s.subscriptionSvc == nil || s.subscriptionSvc.billingCacheService == nil {
		return
	}
	if err := s.subscriptionSvc.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
		s.writeAuditLog(ctx, orderID, "BALANCE_CACHE_INVALIDATE_FAILED", "system", map[string]any{
			"userID": userID,
			"amount": amount,
			"error":  err.Error(),
		})
	}
}

func (s *PaymentService) createZeroSubscriptionUpgradeOrder(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, quote *UpgradeQuote, cfg *PaymentConfig) (*dbent.PaymentOrder, error) {
	now := time.Now()
	return s.createSubscriptionUpgradeOrderInTx(ctx, req, user, plan, quote, cfg, 0, 0, 0, OrderStatusPaid, &now, nil)
}

func (s *PaymentService) createBalanceSubscriptionUpgradeOrderInTx(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, quote *UpgradeQuote, cfg *PaymentConfig, amount float64) (*dbent.PaymentOrder, error) {
	if amount > 0 {
		currentUser, err := s.userRepo.GetByID(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if currentUser.Balance < amount {
			return nil, ErrInsufficientBalance
		}
	}
	now := time.Now()
	return s.createSubscriptionUpgradeOrderInTx(ctx, req, user, plan, quote, cfg, amount, 0, amount, OrderStatusPaid, &now, nil)
}

func (s *PaymentService) createSubscriptionUpgradeOrderInTx(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, quote *UpgradeQuote, cfg *PaymentConfig, amount, feeRate, payAmount float64, initialStatus string, paidAt *time.Time, sel *payment.InstanceSelection) (*dbent.PaymentOrder, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.checkPendingLimit(ctx, tx, req.UserID, cfg.MaxPendingOrders); err != nil {
		return nil, err
	}
	if err := s.ensureNoPendingUpgradeOrder(dbent.NewTxContext(ctx, tx), req.SourceSubscriptionID); err != nil {
		return nil, err
	}
	if req.PaymentType == payment.TypeBalance && amount > 0 {
		affected, err := tx.User.Update().
			Where(dbuser.IDEQ(req.UserID), dbuser.BalanceGTE(amount)).
			AddBalance(-amount).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("deduct balance: %w", err)
		}
		if affected == 0 {
			return nil, ErrInsufficientBalance
		}
	}

	tm := cfg.OrderTimeoutMin
	if tm <= 0 {
		tm = defaultOrderTimeoutMin
	}
	expiresAt := time.Now().Add(time.Duration(tm) * time.Minute)
	if initialStatus == OrderStatusPaid && paidAt != nil {
		expiresAt = *paidAt
	}

	builder := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetNillableUserNotes(psNilIfEmpty(user.Notes)).
		SetAmount(amount).
		SetPayAmount(payAmount).
		SetFeeRate(feeRate).
		SetRechargeCode("").
		SetOutTradeNo(generateOutTradeNo()).
		SetPaymentType(req.PaymentType).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscriptionUpgrade).
		SetPlanID(plan.ID).
		SetSourceSubscriptionID(req.SourceSubscriptionID).
		SetSourcePlanID(quote.SourcePlanID).
		SetSubscriptionGroupID(plan.GroupID).
		SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)).
		SetUpgradeCreditCny(quote.CreditCNY).
		SetUpgradePayableCny(quote.PayableCNY).
		SetUpgradeRemainingRatio(quote.RemainingRatio).
		SetStatus(initialStatus).
		SetExpiresAt(expiresAt).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost)
	if sel != nil {
		if instanceID := strings.TrimSpace(sel.InstanceID); instanceID != "" {
			builder.SetProviderInstanceID(instanceID)
		}
		if providerKey := strings.TrimSpace(sel.ProviderKey); providerKey != "" {
			builder.SetProviderKey(providerKey)
		}
		if snapshot := buildPaymentOrderProviderSnapshot(sel); snapshot != nil {
			builder.SetProviderSnapshot(snapshot)
		}
	}
	if req.SrcURL != "" {
		builder.SetSrcURL(req.SrcURL)
	}
	if paidAt != nil {
		builder.SetPaidAt(*paidAt)
	}
	order, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create upgrade order: %w", err)
	}
	code := fmt.Sprintf("PAY-%d-%d", order.ID, time.Now().UnixNano()%100000)
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetRechargeCode(code).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set recharge code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit order transaction: %w", err)
	}
	s.writeAuditLog(ctx, order.ID, "ORDER_CREATED", fmt.Sprintf("user:%d", req.UserID), map[string]any{
		"orderType":            payment.OrderTypeSubscriptionUpgrade,
		"paymentType":          req.PaymentType,
		"sourceSubscriptionID": req.SourceSubscriptionID,
		"targetPlanID":         plan.ID,
		"payableCNY":           quote.PayableCNY,
		"creditCNY":            quote.CreditCNY,
		"remainingRatio":       quote.RemainingRatio,
	})
	return order, nil
}

func (s *PaymentService) createBalanceSubscriptionOrderInTx(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, amount float64) (*dbent.PaymentOrder, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	affected, err := tx.User.Update().
		Where(dbuser.IDEQ(req.UserID), dbuser.BalanceGTE(amount)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("deduct balance: %w", err)
	}
	if affected == 0 {
		return nil, ErrInsufficientBalance
	}

	now := time.Now()
	order, err := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetNillableUserNotes(psNilIfEmpty(user.Notes)).
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode("").
		SetOutTradeNo(generateOutTradeNo()).
		SetPaymentType(req.PaymentType).
		SetPaymentTradeNo("").
		SetOrderType(req.OrderType).
		SetStatus(OrderStatusRecharging).
		SetExpiresAt(now).
		SetPaidAt(now).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost).
		SetPlanID(plan.ID).
		SetSubscriptionGroupID(plan.GroupID).
		SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	code := fmt.Sprintf("PAY-%d-%d", order.ID, time.Now().UnixNano()%100000)
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetRechargeCode(code).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set recharge code: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit order transaction: %w", err)
	}

	s.writeAuditLog(ctx, order.ID, "ORDER_CREATED", fmt.Sprintf("user:%d", req.UserID), map[string]any{
		"amount":      amount,
		"paymentType": req.PaymentType,
		"orderType":   req.OrderType,
		"source":      "balance",
	})

	return order, nil
}

func selectedStripePublishableKey(sel *payment.InstanceSelection) string {
	if sel == nil || sel.ProviderKey != payment.TypeStripe || sel.Config == nil {
		return ""
	}
	return strings.TrimSpace(sel.Config[payment.ConfigKeyPublishableKey])
}

func (s *PaymentService) buildPaymentSubject(plan *dbent.SubscriptionPlan, payAmountStr string, cfg *PaymentConfig) string {
	if plan != nil {
		if plan.ProductName != "" {
			return plan.ProductName
		}
		return "Sub2API Subscription " + plan.Name
	}
	pf := strings.TrimSpace(cfg.ProductNamePrefix)
	sf := strings.TrimSpace(cfg.ProductNameSuffix)
	if pf != "" || sf != "" {
		return strings.TrimSpace(pf + " " + payAmountStr + " " + sf)
	}
	return "Sub2API " + payAmountStr + " CNY"
}

// --- Order Queries ---

func (s *PaymentService) GetOrder(ctx context.Context, orderID, userID int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this order")
	}
	return o, nil
}

func (s *PaymentService) GetOrderByID(ctx context.Context, orderID int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	return o, nil
}

func (s *PaymentService) GetUserOrders(ctx context.Context, userID int64, p OrderListParams) ([]*dbent.PaymentOrder, int, error) {
	q := s.entClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID))
	if p.Status != "" {
		q = q.Where(paymentorder.StatusEQ(p.Status))
	}
	if p.OrderType != "" {
		q = q.Where(paymentorder.OrderTypeEQ(p.OrderType))
	}
	if p.PaymentType != "" {
		q = q.Where(paymentorder.PaymentTypeEQ(p.PaymentType))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count user orders: %w", err)
	}
	ps, pg := applyPagination(p.PageSize, p.Page)
	orders, err := q.Order(dbent.Desc(paymentorder.FieldCreatedAt)).Limit(ps).Offset((pg - 1) * ps).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query user orders: %w", err)
	}
	return orders, total, nil
}

// AdminListOrders returns a paginated list of orders. If userID > 0, filters by user.
func (s *PaymentService) AdminListOrders(ctx context.Context, userID int64, p OrderListParams) ([]*dbent.PaymentOrder, int, error) {
	q := s.entClient.PaymentOrder.Query()
	if userID > 0 {
		q = q.Where(paymentorder.UserIDEQ(userID))
	}
	if p.Status != "" {
		q = q.Where(paymentorder.StatusEQ(p.Status))
	}
	if p.OrderType != "" {
		q = q.Where(paymentorder.OrderTypeEQ(p.OrderType))
	}
	if p.PaymentType != "" {
		q = q.Where(paymentorder.PaymentTypeEQ(p.PaymentType))
	}
	if p.Keyword != "" {
		q = q.Where(paymentorder.Or(
			paymentorder.OutTradeNoContainsFold(p.Keyword),
			paymentorder.UserEmailContainsFold(p.Keyword),
			paymentorder.UserNameContainsFold(p.Keyword),
		))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count admin orders: %w", err)
	}
	ps, pg := applyPagination(p.PageSize, p.Page)
	orders, err := q.Order(dbent.Desc(paymentorder.FieldCreatedAt)).Limit(ps).Offset((pg - 1) * ps).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query admin orders: %w", err)
	}
	return orders, total, nil
}
