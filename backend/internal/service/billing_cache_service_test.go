package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type billingCacheWorkerStub struct {
	balanceUpdates      int64
	subscriptionUpdates int64
	balance             float64
	balanceErr          error
	subscriptionData    *SubscriptionCacheData
	subscriptionErr     error
	rateLimitData       *APIKeyRateLimitCacheData
	rateLimitErr        error
}

func (b *billingCacheWorkerStub) GetUserBalance(ctx context.Context, userID int64) (float64, error) {
	if b.balanceErr != nil {
		return 0, b.balanceErr
	}
	return b.balance, nil
}

func (b *billingCacheWorkerStub) SetUserBalance(ctx context.Context, userID int64, balance float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) DeductUserBalance(ctx context.Context, userID int64, amount float64) error {
	atomic.AddInt64(&b.balanceUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) InvalidateUserBalance(ctx context.Context, userID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetSubscriptionCache(ctx context.Context, userID, groupID int64) (*SubscriptionCacheData, error) {
	if b.subscriptionErr != nil {
		return nil, b.subscriptionErr
	}
	if b.subscriptionData == nil {
		return nil, errors.New("not implemented")
	}
	return b.subscriptionData, nil
}

func (b *billingCacheWorkerStub) SetSubscriptionCache(ctx context.Context, userID, groupID int64, data *SubscriptionCacheData) error {
	atomic.AddInt64(&b.subscriptionUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) UpdateSubscriptionUsage(ctx context.Context, userID, groupID int64, cost float64) error {
	atomic.AddInt64(&b.subscriptionUpdates, 1)
	return nil
}

func (b *billingCacheWorkerStub) InvalidateSubscriptionCache(ctx context.Context, userID, groupID int64) error {
	return nil
}

func (b *billingCacheWorkerStub) GetAPIKeyRateLimit(ctx context.Context, keyID int64) (*APIKeyRateLimitCacheData, error) {
	if b.rateLimitErr != nil {
		return nil, b.rateLimitErr
	}
	if b.rateLimitData == nil {
		return nil, errors.New("not implemented")
	}
	return b.rateLimitData, nil
}

func (b *billingCacheWorkerStub) SetAPIKeyRateLimit(ctx context.Context, keyID int64, data *APIKeyRateLimitCacheData) error {
	return nil
}

func (b *billingCacheWorkerStub) UpdateAPIKeyRateLimitUsage(ctx context.Context, keyID int64, cost float64) error {
	return nil
}

func (b *billingCacheWorkerStub) InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error {
	return nil
}

func TestBillingCacheServiceQueueHighLoad(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	start := time.Now()
	for i := 0; i < cacheWriteBufferSize*2; i++ {
		svc.QueueDeductBalance(1, 1)
	}
	require.Less(t, time.Since(start), 2*time.Second)

	svc.QueueUpdateSubscriptionUsage(1, 2, 1.5)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&cache.balanceUpdates) > 0
	}, 2*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&cache.subscriptionUpdates) > 0
	}, 2*time.Second, 10*time.Millisecond)
}

func TestBillingCacheServiceEnqueueAfterStopReturnsFalse(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{})
	svc.Stop()

	enqueued := svc.enqueueCacheWrite(cacheWriteTask{
		kind:   cacheWriteDeductBalance,
		userID: 1,
		amount: 1,
	})
	require.False(t, enqueued)
}

func TestBillingCacheServiceSubscriptionLimitGuard(t *testing.T) {
	limit := 100.0
	cache := &billingCacheWorkerStub{
		subscriptionData: &SubscriptionCacheData{
			Status:     SubscriptionStatusActive,
			ExpiresAt:  time.Now().Add(time.Hour),
			DailyUsage: 99.25,
		},
	}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{
		Billing: config.BillingConfig{
			LimitGuard: config.BillingLimitGuardConfig{MinRemainingUSD: 0.5, Percent: 0.01},
		},
	})
	t.Cleanup(svc.Stop)

	err := svc.checkSubscriptionEligibility(context.Background(), 1, &Group{ID: 2, DailyLimitUSD: &limit}, &UserSubscription{ID: 3})

	require.ErrorIs(t, err, ErrDailyLimitExceeded)
}

func TestBillingCacheServiceAPIKeyRateLimitGuard(t *testing.T) {
	now := time.Now()
	cache := &billingCacheWorkerStub{
		rateLimitData: &APIKeyRateLimitCacheData{
			Usage1d:  49.75,
			Window1d: now.Unix(),
		},
	}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{
		Billing: config.BillingConfig{
			LimitGuard: config.BillingLimitGuardConfig{MinRemainingUSD: 0.5, Percent: 0.01},
		},
	})
	t.Cleanup(svc.Stop)

	err := svc.checkAPIKeyRateLimits(context.Background(), &APIKey{ID: 1, RateLimit1d: 50})

	require.ErrorIs(t, err, ErrAPIKeyRateLimit1dExceeded)
}

func TestBillingCacheServiceLimitGuardCappedForSmallLimits(t *testing.T) {
	cache := &billingCacheWorkerStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{
		Billing: config.BillingConfig{
			LimitGuard: config.BillingLimitGuardConfig{MinRemainingUSD: 0.5, Percent: 0.01},
		},
	})
	t.Cleanup(svc.Stop)

	require.False(t, svc.limitGuardExceeded("test", 1, 0.1, 0))
	require.True(t, svc.limitGuardExceeded("test", 1, 0.1, 0.001))
}
