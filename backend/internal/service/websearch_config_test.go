//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type webSearchSettingRepoStub struct {
	getValueFn func(ctx context.Context, key string) (string, error)
}

func (s *webSearchSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *webSearchSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.getValueFn == nil {
		panic("unexpected GetValue call")
	}
	return s.getValueFn(ctx, key)
}

func (s *webSearchSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *webSearchSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *webSearchSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *webSearchSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *webSearchSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func resetWebSearchConfigTestCache(t *testing.T) {
	t.Helper()

	webSearchEmulationCache.Store((*cachedWebSearchEmulationConfig)(nil))
	webSearchEmulationSF.Forget(sfKeyWebSearchConfig)
	t.Cleanup(func() {
		webSearchEmulationCache.Store((*cachedWebSearchEmulationConfig)(nil))
		webSearchEmulationSF.Forget(sfKeyWebSearchConfig)
	})
}

func TestGetWebSearchEmulationConfig_ReturnsDefaultWhenSettingMissing(t *testing.T) {
	resetWebSearchConfigTestCache(t)

	repo := &webSearchSettingRepoStub{
		getValueFn: func(ctx context.Context, key string) (string, error) {
			require.Equal(t, SettingKeyWebSearchEmulationConfig, key)
			return "", ErrSettingNotFound
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	cfg, err := svc.GetWebSearchEmulationConfig(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}

func TestGetWebSearchEmulationConfig_ReturnsErrorOnRepositoryFailure(t *testing.T) {
	resetWebSearchConfigTestCache(t)

	repo := &webSearchSettingRepoStub{
		getValueFn: func(ctx context.Context, key string) (string, error) {
			require.Equal(t, SettingKeyWebSearchEmulationConfig, key)
			return "", errors.New("db down")
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	cfg, err := svc.GetWebSearchEmulationConfig(context.Background())
	require.Error(t, err)
	require.NotNil(t, cfg)
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}
