package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrGatewayRPMExceeded = errors.New("gateway rpm limit exceeded")

type GatewayRPMScope string

const (
	GatewayRPMScopeUser  GatewayRPMScope = "user"
	GatewayRPMScopeGroup GatewayRPMScope = "group"
)

type GatewayRPMExceededError struct {
	Scope GatewayRPMScope
	Limit int
}

func (e *GatewayRPMExceededError) Error() string {
	return fmt.Sprintf("%s rpm limit exceeded: %d rpm", e.Scope, e.Limit)
}

func (e *GatewayRPMExceededError) Unwrap() error { return ErrGatewayRPMExceeded }

type GatewayRPMCache interface {
	Increment(ctx context.Context, key string, window time.Duration) (int64, error)
}

func EnforceGatewayRPM(ctx context.Context, cache GatewayRPMCache, apiKey *APIKey) error {
	if cache == nil || apiKey == nil || apiKey.User == nil {
		return nil
	}
	userID := apiKey.User.ID
	if userID <= 0 {
		return nil
	}
	if limit := limitValue(apiKey.User.RPMLimit); limit > 0 {
		if err := enforceGatewayRPMScope(ctx, cache, GatewayRPMScopeUser, fmt.Sprintf("gateway_rpm:user:%d", userID), limit); err != nil {
			return err
		}
	}
	groupID := int64(0)
	if apiKey.GroupID != nil {
		groupID = *apiKey.GroupID
	} else if apiKey.Group != nil {
		groupID = apiKey.Group.ID
	}
	if groupID <= 0 || apiKey.Group == nil {
		return nil
	}
	groupLimit := apiKey.Group.RPMLimit
	if apiKey.UserGroupRPMLimit != nil {
		groupLimit = apiKey.UserGroupRPMLimit
	}
	if limit := limitValue(groupLimit); limit > 0 {
		if err := enforceGatewayRPMScope(ctx, cache, GatewayRPMScopeGroup, fmt.Sprintf("gateway_rpm:group:%d", groupID), limit); err != nil {
			return err
		}
	}
	return nil
}

func enforceGatewayRPMScope(ctx context.Context, cache GatewayRPMCache, scope GatewayRPMScope, key string, limit int) error {
	count, err := cache.Increment(ctx, key, time.Minute)
	if err != nil {
		return fmt.Errorf("gateway rpm cache: %w", err)
	}
	if count > int64(limit) {
		return &GatewayRPMExceededError{Scope: scope, Limit: limit}
	}
	return nil
}

func limitValue(limit *int) int {
	if limit == nil || *limit <= 0 {
		return 0
	}
	return *limit
}
