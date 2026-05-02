package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccount_IsOpenAIPassthroughEnabled(t *testing.T) {
	t.Run("新字段开启", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"openai_passthrough": true,
			},
		}
		require.True(t, account.IsOpenAIPassthroughEnabled())
	})

	t.Run("兼容旧字段", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_passthrough": true,
			},
		}
		require.True(t, account.IsOpenAIPassthroughEnabled())
	})

	t.Run("非OpenAI账号始终关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_passthrough": true,
			},
		}
		require.False(t, account.IsOpenAIPassthroughEnabled())
	})

	t.Run("空额外配置默认关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		}
		require.False(t, account.IsOpenAIPassthroughEnabled())
	})
}

func TestAccount_IsOpenAIOAuthPassthroughEnabled(t *testing.T) {
	t.Run("仅OAuth类型允许返回开启", func(t *testing.T) {
		oauthAccount := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_passthrough": true,
			},
		}
		require.True(t, oauthAccount.IsOpenAIOAuthPassthroughEnabled())

		apiKeyAccount := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"openai_passthrough": true,
			},
		}
		require.False(t, apiKeyAccount.IsOpenAIOAuthPassthroughEnabled())
	})
}

func TestAccount_IsChatAPIResponsesEnabled(t *testing.T) {
	t.Run("ChatAPI 账号开启新字段时返回 true", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeChatAPI,
			Extra: map[string]any{
				"chatapi_responses_enabled": true,
			},
		}
		require.True(t, account.IsChatAPIResponsesEnabled())
	})

	t.Run("非 ChatAPI 账号始终关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"chatapi_responses_enabled": true,
			},
		}
		require.False(t, account.IsChatAPIResponsesEnabled())
	})

	t.Run("字段缺失或类型错误默认关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeChatAPI,
			Extra: map[string]any{
				"chatapi_responses_enabled": "true",
			},
		}
		require.False(t, account.IsChatAPIResponsesEnabled())
	})
}

func TestAccount_IsCodexCLIOnlyEnabled(t *testing.T) {
	t.Run("OpenAI OAuth 开启", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"codex_cli_only": true,
			},
		}
		require.True(t, account.IsCodexCLIOnlyEnabled())
	})

	t.Run("OpenAI OAuth 关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"codex_cli_only": false,
			},
		}
		require.False(t, account.IsCodexCLIOnlyEnabled())
	})

	t.Run("字段缺失默认关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra:    map[string]any{},
		}
		require.False(t, account.IsCodexCLIOnlyEnabled())
	})

	t.Run("类型非法默认关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"codex_cli_only": "true",
			},
		}
		require.False(t, account.IsCodexCLIOnlyEnabled())
	})

	t.Run("非 OAuth 账号始终关闭", func(t *testing.T) {
		apiKeyAccount := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"codex_cli_only": true,
			},
		}
		require.False(t, apiKeyAccount.IsCodexCLIOnlyEnabled())

		otherPlatform := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"codex_cli_only": true,
			},
		}
		require.False(t, otherPlatform.IsCodexCLIOnlyEnabled())
	})
}

func TestAccount_IsOpenAIResponsesWebSocketV2Enabled(t *testing.T) {
	t.Run("OAuth使用OAuth专用开关", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_responses_websockets_v2_enabled": true,
			},
		}
		require.True(t, account.IsOpenAIResponsesWebSocketV2Enabled())
	})

	t.Run("API Key使用API Key专用开关", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		}
		require.True(t, account.IsOpenAIResponsesWebSocketV2Enabled())
	})

	t.Run("OAuth账号不会读取API Key专用开关", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
			},
		}
		require.False(t, account.IsOpenAIResponsesWebSocketV2Enabled())
	})

	t.Run("分类型新键优先于兼容键", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_responses_websockets_v2_enabled": false,
				"responses_websockets_v2_enabled":              true,
				"openai_ws_enabled":                            true,
			},
		}
		require.False(t, account.IsOpenAIResponsesWebSocketV2Enabled())
	})

	t.Run("分类型键缺失时回退兼容键", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"responses_websockets_v2_enabled": true,
			},
		}
		require.True(t, account.IsOpenAIResponsesWebSocketV2Enabled())
	})

	t.Run("非OpenAI账号默认关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"responses_websockets_v2_enabled": true,
			},
		}
		require.False(t, account.IsOpenAIResponsesWebSocketV2Enabled())
	})
}

func TestAccount_IsOpenAIOAuthImagesExperimentalEnabled(t *testing.T) {
	t.Run("仅OpenAI OAuth读取账号级开关", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_images_experimental": true,
			},
		}
		require.True(t, account.IsOpenAIOAuthImagesExperimentalEnabled())
	})

	t.Run("非OAuth或非OpenAI账号默认关闭", func(t *testing.T) {
		require.False(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_oauth_images_experimental": true}}).IsOpenAIOAuthImagesExperimentalEnabled())
		require.False(t, (&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"openai_oauth_images_experimental": true}}).IsOpenAIOAuthImagesExperimentalEnabled())
	})
}

func TestAccount_OpenAIOAuthImagesProbeFields(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_images_probe_supported": true,
			"openai_oauth_images_strategy":        "API_PLATFORM_IMAGES_WITH_OAUTH",
			"openai_oauth_images_probe_reason":    "manual_probe_passed",
			"openai_oauth_images_probe_status":    204,
		},
	}

	require.True(t, account.IsOpenAIOAuthImagesProbeSupported())
	require.Equal(t, "api_platform_images_with_oauth", account.OpenAIOAuthImagesStrategy())
	require.Equal(t, "manual_probe_passed", account.OpenAIOAuthImagesProbeReason())
	require.Equal(t, 204, account.OpenAIOAuthImagesProbeStatus())
}

func TestAccount_ResolveOpenAIResponsesWebSocketV2Mode(t *testing.T) {
	t.Run("default fallback to ctx_pool", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra:    map[string]any{},
		}
		require.Equal(t, OpenAIWSIngressModeCtxPool, account.ResolveOpenAIResponsesWebSocketV2Mode(""))
		require.Equal(t, OpenAIWSIngressModeCtxPool, account.ResolveOpenAIResponsesWebSocketV2Mode("invalid"))
	})

	t.Run("oauth mode field has highest priority", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_responses_websockets_v2_mode":    OpenAIWSIngressModePassthrough,
				"openai_oauth_responses_websockets_v2_enabled": false,
				"responses_websockets_v2_enabled":              false,
			},
		}
		require.Equal(t, OpenAIWSIngressModePassthrough, account.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeCtxPool))
	})

	t.Run("legacy enabled maps to ctx_pool", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"responses_websockets_v2_enabled": true,
			},
		}
		require.Equal(t, OpenAIWSIngressModeCtxPool, account.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeOff))
	})

	t.Run("shared/dedicated mode strings are compatible with ctx_pool", func(t *testing.T) {
		shared := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeShared,
			},
		}
		dedicated := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeDedicated,
			},
		}
		require.Equal(t, OpenAIWSIngressModeShared, shared.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeOff))
		require.Equal(t, OpenAIWSIngressModeDedicated, dedicated.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeOff))
		require.Equal(t, OpenAIWSIngressModeCtxPool, normalizeOpenAIWSIngressDefaultMode(OpenAIWSIngressModeShared))
		require.Equal(t, OpenAIWSIngressModeCtxPool, normalizeOpenAIWSIngressDefaultMode(OpenAIWSIngressModeDedicated))
	})

	t.Run("legacy disabled maps to off", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": false,
				"responses_websockets_v2_enabled":               true,
			},
		}
		require.Equal(t, OpenAIWSIngressModeOff, account.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeCtxPool))
	})

	t.Run("non openai always off", func(t *testing.T) {
		account := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeDedicated,
			},
		}
		require.Equal(t, OpenAIWSIngressModeOff, account.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeDedicated))
	})
}

func TestAccount_OpenAIWSExtraFlags(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_ws_force_http":           true,
			"openai_ws_allow_store_recovery": true,
		},
	}
	require.True(t, account.IsOpenAIWSForceHTTPEnabled())
	require.True(t, account.IsOpenAIWSAllowStoreRecoveryEnabled())

	off := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{}}
	require.False(t, off.IsOpenAIWSForceHTTPEnabled())
	require.False(t, off.IsOpenAIWSAllowStoreRecoveryEnabled())

	var nilAccount *Account
	require.False(t, nilAccount.IsOpenAIWSAllowStoreRecoveryEnabled())

	nonOpenAI := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_ws_allow_store_recovery": true,
		},
	}
	require.False(t, nonOpenAI.IsOpenAIWSAllowStoreRecoveryEnabled())
}
