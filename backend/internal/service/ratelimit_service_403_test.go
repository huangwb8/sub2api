//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type openAI403CounterStub struct {
	count       int64
	increments  int
	resetCalled int
}

func (s *openAI403CounterStub) IncrementOpenAI403Count(ctx context.Context, accountID int64, windowMinutes int) (int64, error) {
	s.increments++
	return s.count, nil
}

func (s *openAI403CounterStub) ResetOpenAI403Count(ctx context.Context, accountID int64) error {
	s.resetCalled++
	return nil
}

func TestRateLimitService_HandleUpstreamError_OpenAI403UsesTempCooldownBeforeThreshold(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterStub{count: 1}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetOpenAI403CounterCache(counter)

	account := &Account{ID: 1001, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{}, []byte(`{"error":{"message":"temporary forbidden"}}`))

	require.True(t, shouldDisable)
	require.Equal(t, 1, counter.increments)
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, 0, repo.setErrorCalls)
}

func TestRateLimitService_HandleUpstreamError_OpenAI403DisablesAtThreshold(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := &openAI403CounterStub{count: openAI403DisableThreshold}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetOpenAI403CounterCache(counter)

	account := &Account{ID: 1002, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{}, []byte(`{"error":{"message":"permanent forbidden"}}`))

	require.True(t, shouldDisable)
	require.Equal(t, 1, counter.increments)
	require.Equal(t, 0, repo.tempCalls)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Contains(t, repo.lastErrorMsg, "consecutive_403")
}
