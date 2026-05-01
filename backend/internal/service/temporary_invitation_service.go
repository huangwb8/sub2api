package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const temporaryInvitationSweepTimeout = 30 * time.Second

type TemporaryInvitationSweepResult struct {
	Normalized int
	Disabled   int
	Deleted    int
}

type TemporaryInvitationService struct {
	entClient            *dbent.Client
	authCacheInvalidator APIKeyAuthCacheInvalidator
	interval             time.Duration
	now                  func() time.Time
	stopCh               chan struct{}
	stopOnce             sync.Once
	wg                   sync.WaitGroup
}

func NewTemporaryInvitationService(entClient *dbent.Client, authCacheInvalidator APIKeyAuthCacheInvalidator, interval time.Duration) *TemporaryInvitationService {
	return &TemporaryInvitationService{
		entClient:            entClient,
		authCacheInvalidator: authCacheInvalidator,
		interval:             interval,
		now:                  time.Now,
		stopCh:               make(chan struct{}),
	}
}

func (s *TemporaryInvitationService) Start() {
	if s == nil || s.entClient == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *TemporaryInvitationService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *TemporaryInvitationService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), temporaryInvitationSweepTimeout)
	defer cancel()

	result, err := s.RunOnce(ctx)
	if err != nil {
		slog.Error("[TemporaryInvitation] sweep failed", "error", err)
		return
	}
	if result.Normalized > 0 || result.Disabled > 0 || result.Deleted > 0 {
		slog.Info(
			"[TemporaryInvitation] sweep completed",
			"normalized", result.Normalized,
			"disabled", result.Disabled,
			"deleted", result.Deleted,
		)
	}
}

func (s *TemporaryInvitationService) RunOnce(ctx context.Context) (TemporaryInvitationSweepResult, error) {
	var result TemporaryInvitationSweepResult
	if s == nil || s.entClient == nil {
		return result, nil
	}

	users, err := s.entClient.User.Query().
		Where(user.TemporaryInvitationEQ(true)).
		All(ctx)
	if err != nil {
		return result, err
	}

	now := s.now()
	for _, item := range users {
		qualifiedAmount, err := s.sumQualifiedRechargeAmount(ctx, item.ID)
		if err != nil {
			slog.Error("[TemporaryInvitation] failed to inspect recharge amount", "user_id", item.ID, "error", err)
			continue
		}

		switch {
		case TemporaryInvitationQualified(qualifiedAmount):
			changed, normalizeErr := s.normalizeQualifiedUser(ctx, item)
			if normalizeErr != nil {
				slog.Error("[TemporaryInvitation] failed to normalize qualified user", "user_id", item.ID, "error", normalizeErr)
				continue
			}
			if changed {
				result.Normalized++
			}
		case item.TemporaryInvitationDisabledAt == nil && item.TemporaryInvitationDeadlineAt != nil && !item.TemporaryInvitationDeadlineAt.After(now):
			changed, disableErr := s.disableExpiredUser(ctx, item.ID, now)
			if disableErr != nil {
				slog.Error("[TemporaryInvitation] failed to disable expired user", "user_id", item.ID, "error", disableErr)
				continue
			}
			if changed {
				result.Disabled++
			}
		case item.TemporaryInvitationDisabledAt != nil && item.TemporaryInvitationDeleteAt != nil && !item.TemporaryInvitationDeleteAt.After(now):
			deleted, deleteErr := s.hardDeleteUser(ctx, item.ID)
			if deleteErr != nil {
				slog.Error("[TemporaryInvitation] failed to hard delete expired user", "user_id", item.ID, "error", deleteErr)
				continue
			}
			if deleted {
				result.Deleted++
			}
		}
	}

	return result, nil
}

func (s *TemporaryInvitationService) sumQualifiedRechargeAmount(ctx context.Context, userID int64) (float64, error) {
	var rows []struct {
		Sum float64 `json:"sum"`
	}
	err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(userID),
			paymentorder.OrderTypeEQ(payment.OrderTypeBalance),
			paymentorder.StatusIn(OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted),
		).
		Aggregate(dbent.As(dbent.Sum(paymentorder.FieldAmount), "sum")).
		Scan(ctx, &rows)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Sum, nil
}

func (s *TemporaryInvitationService) normalizeQualifiedUser(ctx context.Context, item *dbent.User) (bool, error) {
	if item == nil || !item.TemporaryInvitation {
		return false, nil
	}
	update := s.entClient.User.UpdateOneID(item.ID).
		SetTemporaryInvitation(false).
		ClearTemporaryInvitationDeadlineAt().
		ClearTemporaryInvitationDisabledAt().
		ClearTemporaryInvitationDeleteAt()
	if _, err := update.Save(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TemporaryInvitationService) disableExpiredUser(ctx context.Context, userID int64, now time.Time) (bool, error) {
	deleteAt := now.Add(TemporaryInvitationDeleteWindow)
	updated, err := s.entClient.User.Update().
		Where(
			user.IDEQ(userID),
			user.TemporaryInvitationEQ(true),
			user.TemporaryInvitationDisabledAtIsNil(),
		).
		SetStatus(StatusDisabled).
		SetTemporaryInvitationDisabledAt(now).
		SetTemporaryInvitationDeleteAt(deleteAt).
		Save(ctx)
	if err != nil {
		return false, err
	}
	if updated == 0 {
		return false, nil
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return true, nil
}

func (s *TemporaryInvitationService) hardDeleteUser(ctx context.Context, userID int64) (bool, error) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	affected, err := s.entClient.User.Delete().
		Where(user.IDEQ(userID)).
		Exec(mixins.SkipSoftDelete(ctx))
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
