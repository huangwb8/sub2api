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

	t.Run("new global flag explicitly disabled", func(t *testing.T) {
		disabled := false
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					OpenAIOAuthImagesEnabled: &disabled,
				},
			},
		}
		_, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental":    true,
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "generations", false)
		typedErr, ok := ResolveOpenAIOAuthImagesError(err)
		require.True(t, ok)
		require.Equal(t, "oauth_images_disabled", typedErr.Code)
	})

	t.Run("account probe fields are no longer required", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{},
			},
		}
		capability, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "generations", false)
		require.NoError(t, err)
		require.NotNil(t, capability)
		require.Equal(t, OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool, capability.Strategy)
	})

	t.Run("probe failure metadata no longer blocks", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{},
			},
		}
		capability, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental": true,
			"openai_oauth_images_strategy":     OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
			"openai_oauth_images_probe_reason": "probe_403_unsupported",
			"openai_oauth_images_probe_status": 403,
		}), "generations", false)
		require.NoError(t, err)
		require.NotNil(t, capability)
		require.True(t, capability.Supported)
	})

	t.Run("stream supported through responses bridge", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{},
			},
		}
		capability, err := svc.ValidateOpenAIImagesAccount(ctx, newAccount(map[string]any{
			"openai_oauth_images_experimental":    true,
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth,
		}), "generations", true)
		require.NoError(t, err)
		require.NotNil(t, capability)
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
		require.Equal(t, OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool, capability.Strategy)

		cachedAny, ok := svc.openaiOAuthImagesCapabilities.Load(account.ID)
		require.True(t, ok)
		cached, ok := cachedAny.(OpenAIOAuthImagesCapability)
		require.True(t, ok)
		require.True(t, time.Since(cached.CheckedAt) < time.Minute)
		require.Equal(t, 10*time.Minute, cached.TTL)
	})
}
