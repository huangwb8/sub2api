package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_ValidateOpenAIImagesAccount(t *testing.T) {
	ctx := context.Background()
	newAccount := func(extra map[string]any) *Account {
		return &Account{
			ID:       101,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra:    extra,
		}
	}

	t.Run("API Key remains supported", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		capability, err := svc.ValidateOpenAIImagesAccount(ctx, &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, "generations", false)
		require.NoError(t, err)
		require.Nil(t, capability)
	})

	t.Run("global flag disabled", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{},
			},
		}
		_, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental":    true,
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "generations", false)
		typedErr, ok := ResolveOpenAIOAuthImagesError(err)
		require.True(t, ok)
		require.Equal(t, "oauth_images_experimental_disabled", typedErr.Code)
	})

	t.Run("account flag disabled", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					OpenAIOAuthImagesExperimentalEnabled: true,
				},
			},
		}
		_, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "generations", false)
		typedErr, ok := ResolveOpenAIOAuthImagesError(err)
		require.True(t, ok)
		require.Equal(t, "oauth_images_account_disabled", typedErr.Code)
	})

	t.Run("probe not supported", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					OpenAIOAuthImagesExperimentalEnabled: true,
				},
			},
		}
		_, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental": true,
			"openai_oauth_images_strategy":     OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
			"openai_oauth_images_probe_reason": "probe_403_unsupported",
			"openai_oauth_images_probe_status": 403,
		}), "generations", false)
		typedErr, ok := ResolveOpenAIOAuthImagesError(err)
		require.True(t, ok)
		require.Equal(t, "oauth_images_probe_failed", typedErr.Code)
		require.Contains(t, typedErr.Message, "probe_403_unsupported")
	})

	t.Run("stream not supported", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					OpenAIOAuthImagesExperimentalEnabled: true,
				},
			},
		}
		_, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental":    true,
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "generations", true)
		typedErr, ok := ResolveOpenAIOAuthImagesError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusBadRequest, typedErr.Status)
		require.Equal(t, "oauth_images_stream_not_supported", typedErr.Code)
	})

	t.Run("edits not supported", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					OpenAIOAuthImagesExperimentalEnabled: true,
				},
			},
		}
		_, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental":    true,
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "edits", false)
		typedErr, ok := ResolveOpenAIOAuthImagesError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusNotImplemented, typedErr.Status)
		require.Equal(t, "oauth_images_edits_not_supported", typedErr.Code)
	})

	t.Run("supported capability is cached", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					OpenAIOAuthImagesExperimentalEnabled: true,
					OpenAIOAuthImagesProbeTTLSeconds:     600,
				},
			},
		}
		account := newAccount(map[string]any{
			"openai_oauth_images_experimental":    true,
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		})
		capability, err := svc.ValidateOpenAIImagesAccount(ctx, account, "generations", false)
		require.NoError(t, err)
		require.NotNil(t, capability)
		require.True(t, capability.Supported)
		require.Equal(t, OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth, capability.Strategy)

		cachedAny, ok := svc.openaiOAuthImagesCapabilities.Load(account.ID)
		require.True(t, ok)
		cached, ok := cachedAny.(OpenAIOAuthImagesCapability)
		require.True(t, ok)
		require.True(t, time.Since(cached.CheckedAt) < time.Minute)
		require.Equal(t, 10*time.Minute, cached.TTL)
	})
}
