//go:build unit

package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbgroup "github.com/Wei-Shaw/sub2api/ent/group"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	dbusersubscription "github.com/Wei-Shaw/sub2api/ent/usersubscription"
)

func paymentTestClientFromContext(ctx context.Context, client *dbent.Client) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return client
}

type paymentTestUserRepo struct {
	UserRepository
	client *dbent.Client
}

func (r *paymentTestUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	user, err := paymentTestClientFromContext(ctx, r.client).User.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &User{
		ID:           user.ID,
		Email:        user.Email,
		Username:     user.Username,
		Notes:        user.Notes,
		PasswordHash: user.PasswordHash,
		Role:         user.Role,
		Balance:      user.Balance,
		Concurrency:  user.Concurrency,
		Status:       user.Status,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}, nil
}

func (r *paymentTestUserRepo) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	updated, err := paymentTestClientFromContext(ctx, r.client).User.Update().
		Where(dbuser.IDEQ(id)).
		AddBalance(amount).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *paymentTestUserRepo) DeductBalance(ctx context.Context, id int64, amount float64) error {
	updated, err := paymentTestClientFromContext(ctx, r.client).User.Update().
		Where(dbuser.IDEQ(id)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrUserNotFound
	}
	return nil
}

type paymentTestGroupRepo struct {
	GroupRepository
	client *dbent.Client
}

func (r *paymentTestGroupRepo) GetByID(ctx context.Context, id int64) (*Group, error) {
	group, err := paymentTestClientFromContext(ctx, r.client).Group.Query().
		Where(dbgroup.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}

	return &Group{
		ID:               group.ID,
		Name:             group.Name,
		Platform:         group.Platform,
		RateMultiplier:   group.RateMultiplier,
		Status:           group.Status,
		Hydrated:         true,
		SubscriptionType: group.SubscriptionType,
		CreatedAt:        group.CreatedAt,
		UpdatedAt:        group.UpdatedAt,
	}, nil
}

type paymentTestUserSubscriptionRepo struct {
	UserSubscriptionRepository
	client *dbent.Client
}

func (r *paymentTestUserSubscriptionRepo) Create(ctx context.Context, sub *UserSubscription) error {
	if sub == nil {
		return ErrSubscriptionNilInput
	}

	created, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.Create().
		SetUserID(sub.UserID).
		SetGroupID(sub.GroupID).
		SetNillableCurrentPlanID(sub.CurrentPlanID).
		SetCurrentPlanName(sub.CurrentPlanName).
		SetNillableCurrentPlanPriceCny(sub.CurrentPlanPriceCNY).
		SetNillableCurrentPlanValidityDays(sub.CurrentPlanValidityDays).
		SetCurrentPlanValidityUnit(sub.CurrentPlanValidityUnit).
		SetNillableBillingCycleStartedAt(sub.BillingCycleStartedAt).
		SetStartsAt(sub.StartsAt).
		SetExpiresAt(sub.ExpiresAt).
		SetStatus(sub.Status).
		SetNillableAssignedBy(sub.AssignedBy).
		SetAssignedAt(sub.AssignedAt).
		SetNotes(sub.Notes).
		Save(ctx)
	if err != nil {
		return err
	}

	applyPaymentTestUserSubscription(sub, created)
	return nil
}

func (r *paymentTestUserSubscriptionRepo) GetByID(ctx context.Context, id int64) (*UserSubscription, error) {
	sub, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.Query().
		Where(dbusersubscription.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return paymentTestUserSubscriptionToService(sub), nil
}

func (r *paymentTestUserSubscriptionRepo) GetByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	sub, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.Query().
		Where(
			dbusersubscription.UserIDEQ(userID),
			dbusersubscription.GroupIDEQ(groupID),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return paymentTestUserSubscriptionToService(sub), nil
}

func (r *paymentTestUserSubscriptionRepo) GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*UserSubscription, error) {
	sub, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.Query().
		Where(
			dbusersubscription.UserIDEQ(userID),
			dbusersubscription.GroupIDEQ(groupID),
			dbusersubscription.StatusEQ(SubscriptionStatusActive),
			dbusersubscription.ExpiresAtGT(time.Now()),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	return paymentTestUserSubscriptionToService(sub), nil
}

func (r *paymentTestUserSubscriptionRepo) Update(ctx context.Context, sub *UserSubscription) error {
	if sub == nil {
		return ErrSubscriptionNilInput
	}

	updated, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.UpdateOneID(sub.ID).
		SetUserID(sub.UserID).
		SetGroupID(sub.GroupID).
		SetNillableCurrentPlanID(sub.CurrentPlanID).
		SetCurrentPlanName(sub.CurrentPlanName).
		SetNillableCurrentPlanPriceCny(sub.CurrentPlanPriceCNY).
		SetNillableCurrentPlanValidityDays(sub.CurrentPlanValidityDays).
		SetCurrentPlanValidityUnit(sub.CurrentPlanValidityUnit).
		SetNillableBillingCycleStartedAt(sub.BillingCycleStartedAt).
		SetStartsAt(sub.StartsAt).
		SetExpiresAt(sub.ExpiresAt).
		SetStatus(sub.Status).
		SetNillableAssignedBy(sub.AssignedBy).
		SetAssignedAt(sub.AssignedAt).
		SetNotes(sub.Notes).
		Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrSubscriptionNotFound
		}
		return err
	}

	applyPaymentTestUserSubscription(sub, updated)
	return nil
}

func (r *paymentTestUserSubscriptionRepo) Delete(ctx context.Context, id int64) error {
	_, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.Delete().
		Where(dbusersubscription.IDEQ(id)).
		Exec(ctx)
	return err
}

func (r *paymentTestUserSubscriptionRepo) ExtendExpiry(ctx context.Context, subscriptionID int64, newExpiresAt time.Time) error {
	_, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.UpdateOneID(subscriptionID).
		SetExpiresAt(newExpiresAt).
		Save(ctx)
	if err != nil && dbent.IsNotFound(err) {
		return ErrSubscriptionNotFound
	}
	return err
}

func (r *paymentTestUserSubscriptionRepo) UpdateStatus(ctx context.Context, subscriptionID int64, status string) error {
	_, err := paymentTestClientFromContext(ctx, r.client).UserSubscription.UpdateOneID(subscriptionID).
		SetStatus(status).
		Save(ctx)
	if err != nil && dbent.IsNotFound(err) {
		return ErrSubscriptionNotFound
	}
	return err
}

func paymentTestUserSubscriptionToService(sub *dbent.UserSubscription) *UserSubscription {
	if sub == nil {
		return nil
	}

	notes := ""
	if sub.Notes != nil {
		notes = *sub.Notes
	}

	return &UserSubscription{
		ID:                      sub.ID,
		UserID:                  sub.UserID,
		GroupID:                 sub.GroupID,
		CurrentPlanID:           sub.CurrentPlanID,
		CurrentPlanName:         sub.CurrentPlanName,
		CurrentPlanPriceCNY:     sub.CurrentPlanPriceCny,
		CurrentPlanValidityDays: sub.CurrentPlanValidityDays,
		CurrentPlanValidityUnit: sub.CurrentPlanValidityUnit,
		BillingCycleStartedAt:   sub.BillingCycleStartedAt,
		StartsAt:                sub.StartsAt,
		ExpiresAt:               sub.ExpiresAt,
		Status:                  sub.Status,
		DailyWindowStart:        sub.DailyWindowStart,
		WeeklyWindowStart:       sub.WeeklyWindowStart,
		MonthlyWindowStart:      sub.MonthlyWindowStart,
		DailyUsageUSD:           sub.DailyUsageUsd,
		WeeklyUsageUSD:          sub.WeeklyUsageUsd,
		MonthlyUsageUSD:         sub.MonthlyUsageUsd,
		AssignedBy:              sub.AssignedBy,
		AssignedAt:              sub.AssignedAt,
		Notes:                   notes,
		CreatedAt:               sub.CreatedAt,
		UpdatedAt:               sub.UpdatedAt,
	}
}

func applyPaymentTestUserSubscription(dst *UserSubscription, src *dbent.UserSubscription) {
	if dst == nil || src == nil {
		return
	}

	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.CurrentPlanID = src.CurrentPlanID
	dst.CurrentPlanName = src.CurrentPlanName
	dst.CurrentPlanPriceCNY = src.CurrentPlanPriceCny
	dst.CurrentPlanValidityDays = src.CurrentPlanValidityDays
	dst.CurrentPlanValidityUnit = src.CurrentPlanValidityUnit
	dst.BillingCycleStartedAt = src.BillingCycleStartedAt
}
