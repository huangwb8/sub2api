package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth = "api_platform_images_with_oauth"
	OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool  = "chatgpt_codex_responses_tool"
	OpenAIOAuthImagesStrategyChatGPTInternalImages      = "chatgpt_internal_images"

	openAIOAuthImagesProbeReasonExperimentalDisabled = "oauth_images_experimental_disabled"
	openAIOAuthImagesProbeReasonAccountDisabled      = "oauth_images_account_disabled"
	openAIOAuthImagesProbeReasonProbeFailed          = "oauth_images_probe_failed"
	openAIOAuthImagesProbeReasonStrategyUnsupported  = "oauth_images_strategy_unsupported"
)

type OpenAIOAuthImagesCapability struct {
	Supported bool
	Strategy  string
	CheckedAt time.Time
	TTL       time.Duration
	Status    int
	Reason    string
}

type openAIOAuthImagesError struct {
	Status  int
	ErrType string
	Code    string
	Message string
}

func (e *openAIOAuthImagesError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Code) == "" {
		return strings.TrimSpace(e.Message)
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(e.Code), strings.TrimSpace(e.Message))
}

func newOpenAIOAuthImagesError(status int, errType, code, message string) *openAIOAuthImagesError {
	return &openAIOAuthImagesError{
		Status:  status,
		ErrType: strings.TrimSpace(errType),
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
	}
}

func (s *OpenAIGatewayService) openAIOAuthImagesProbeTTL() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIOAuthImagesProbeTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.OpenAIOAuthImagesProbeTTLSeconds) * time.Second
	}
	return 10 * time.Minute
}

func (s *OpenAIGatewayService) isOpenAIOAuthImagesExperimentalEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIOAuthImagesExperimentalEnabled
}

func (s *OpenAIGatewayService) isOpenAIOAuthImagesEnabled() bool {
	if s == nil || s.cfg == nil {
		return true
	}
	if s.cfg.Gateway.OpenAIOAuthImagesEnabled != nil {
		return *s.cfg.Gateway.OpenAIOAuthImagesEnabled
	}
	return true
}

func normalizeOpenAIOAuthImagesStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool:
		return OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool
	case OpenAIOAuthImagesStrategyAPIPlatformImagesWithOAuth:
		return OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool
	case OpenAIOAuthImagesStrategyChatGPTInternalImages:
		return OpenAIOAuthImagesStrategyChatGPTInternalImages
	default:
		return ""
	}
}

func (s *OpenAIGatewayService) getOpenAIOAuthImagesCapability(_ context.Context, account *Account) OpenAIOAuthImagesCapability {
	now := time.Now()
	ttl := s.openAIOAuthImagesProbeTTL()
	if account == nil || !account.IsOpenAIOAuth() {
		return OpenAIOAuthImagesCapability{
			CheckedAt: now,
			TTL:       ttl,
			Status:    http.StatusBadRequest,
			Reason:    "oauth_images_account_type_unsupported",
		}
	}

	if cached, ok := s.openaiOAuthImagesCapabilities.Load(account.ID); ok {
		if capability, ok := cached.(OpenAIOAuthImagesCapability); ok {
			if capability.TTL <= 0 {
				capability.TTL = ttl
			}
			if !capability.CheckedAt.IsZero() && now.Sub(capability.CheckedAt) < capability.TTL {
				return capability
			}
		}
	}

	capability := OpenAIOAuthImagesCapability{
		CheckedAt: now,
		TTL:       ttl,
		Status:    http.StatusServiceUnavailable,
		Reason:    openAIOAuthImagesProbeReasonProbeFailed,
	}

	switch {
	case !s.isOpenAIOAuthImagesEnabled():
		capability.Reason = openAIOAuthImagesProbeReasonExperimentalDisabled
	default:
		strategy := normalizeOpenAIOAuthImagesStrategy(account.OpenAIOAuthImagesStrategy())
		if strategy == "" {
			capability.Reason = openAIOAuthImagesProbeReasonStrategyUnsupported
			break
		}
		capability.Strategy = strategy
		switch strategy {
		case OpenAIOAuthImagesStrategyChatGPTCodexResponsesTool:
			capability.Supported = true
			capability.Status = http.StatusOK
			capability.Reason = "ok"
		default:
			capability.Status = http.StatusNotImplemented
			capability.Reason = openAIOAuthImagesProbeReasonStrategyUnsupported
		}
	}

	s.openaiOAuthImagesCapabilities.Store(account.ID, capability)
	return capability
}

func (s *OpenAIGatewayService) ValidateOpenAIImagesAccount(ctx context.Context, account *Account, operation string, reqStream bool) (*OpenAIOAuthImagesCapability, error) {
	_ = reqStream
	if account == nil {
		return nil, newOpenAIOAuthImagesError(http.StatusBadRequest, "invalid_request_error", "oauth_images_account_missing", "No upstream account selected")
	}
	if account.IsOpenAIApiKey() {
		return nil, nil
	}
	if !account.IsOpenAIOAuth() {
		return nil, newOpenAIOAuthImagesError(http.StatusServiceUnavailable, "api_error", "oauth_images_account_type_unsupported", "Images API currently only supports OpenAI API Key accounts or explicitly enabled OAuth experimental accounts")
	}
	if operation != "generations" {
		return nil, newOpenAIOAuthImagesError(http.StatusNotImplemented, "api_error", "oauth_images_edits_not_supported", "OpenAI OAuth image bridge currently only supports /v1/images/generations")
	}

	capability := s.getOpenAIOAuthImagesCapability(ctx, account)
	if capability.Supported {
		return &capability, nil
	}

	switch capability.Reason {
	case openAIOAuthImagesProbeReasonExperimentalDisabled:
		return nil, newOpenAIOAuthImagesError(http.StatusServiceUnavailable, "api_error", "oauth_images_disabled", "OpenAI OAuth image generation is disabled")
	case openAIOAuthImagesProbeReasonAccountDisabled:
		return nil, newOpenAIOAuthImagesError(http.StatusServiceUnavailable, "api_error", "oauth_images_account_disabled", "This OpenAI OAuth account is not enabled for image generation")
	case openAIOAuthImagesProbeReasonStrategyUnsupported:
		return nil, newOpenAIOAuthImagesError(http.StatusServiceUnavailable, "api_error", "oauth_images_strategy_unsupported", "The configured OpenAI OAuth image strategy is not implemented in this deployment")
	default:
		msg := "OpenAI OAuth image generation bridge did not confirm upstream support"
		if reason := strings.TrimSpace(capability.Reason); reason != "" && reason != openAIOAuthImagesProbeReasonProbeFailed {
			msg = msg + ": " + reason
		}
		return nil, newOpenAIOAuthImagesError(http.StatusServiceUnavailable, "api_error", "oauth_images_probe_failed", msg)
	}
}

func ResolveOpenAIOAuthImagesError(err error) (*openAIOAuthImagesError, bool) {
	if err == nil {
		return nil, false
	}
	var target *openAIOAuthImagesError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
