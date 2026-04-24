//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type gatewayRPMCacheStub struct {
	counts map[string]int64
	err    error
}

func (s *gatewayRPMCacheStub) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.counts == nil {
		s.counts = make(map[string]int64)
	}
	s.counts[key]++
	return s.counts[key], nil
}

func TestEnforceGatewayRPM_UserAndGroupScopes(t *testing.T) {
	userLimit := 2
	groupLimit := 1
	groupID := int64(20)
	apiKey := &APIKey{
		User:    &User{ID: 10, RPMLimit: &userLimit},
		GroupID: &groupID,
		Group:   &Group{ID: groupID, RPMLimit: &groupLimit},
	}
	cache := &gatewayRPMCacheStub{}

	require.NoError(t, EnforceGatewayRPM(context.Background(), cache, apiKey))
	err := EnforceGatewayRPM(context.Background(), cache, apiKey)
	require.ErrorIs(t, err, ErrGatewayRPMExceeded)
	var exceeded *GatewayRPMExceededError
	require.True(t, errors.As(err, &exceeded))
	require.Equal(t, GatewayRPMScopeGroup, exceeded.Scope)
}

func TestEnforceGatewayRPM_ZeroAndNilAreUnlimited(t *testing.T) {
	zero := 0
	groupID := int64(20)
	apiKey := &APIKey{
		User:    &User{ID: 10, RPMLimit: &zero},
		GroupID: &groupID,
		Group:   &Group{ID: groupID},
	}
	cache := &gatewayRPMCacheStub{}

	for range 3 {
		require.NoError(t, EnforceGatewayRPM(context.Background(), cache, apiKey))
	}
	require.Empty(t, cache.counts)
}

func TestEnforceGatewayRPM_UserGroupOverrideReplacesGroupLimit(t *testing.T) {
	groupLimit := 1
	overrideLimit := 0
	groupID := int64(20)
	apiKey := &APIKey{
		User:              &User{ID: 10},
		GroupID:           &groupID,
		Group:             &Group{ID: groupID, RPMLimit: &groupLimit},
		UserGroupRPMLimit: &overrideLimit,
	}
	cache := &gatewayRPMCacheStub{}

	for range 3 {
		require.NoError(t, EnforceGatewayRPM(context.Background(), cache, apiKey))
	}
	require.Empty(t, cache.counts)
}
