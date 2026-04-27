package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrTurnstileVerificationFailed = infraerrors.BadRequest("TURNSTILE_VERIFICATION_FAILED", "turnstile verification failed")
	ErrTurnstileNotConfigured      = infraerrors.ServiceUnavailable("TURNSTILE_NOT_CONFIGURED", "turnstile not configured")
	ErrTurnstileInvalidSecretKey   = infraerrors.BadRequest("TURNSTILE_INVALID_SECRET_KEY", "invalid turnstile secret key")
)

// TurnstileVerifier 验证 Turnstile token 的接口
type TurnstileVerifier interface {
	VerifyToken(ctx context.Context, secretKey, token, remoteIP string) (*TurnstileVerifyResponse, error)
}

// TurnstileService Turnstile 验证服务
type TurnstileService struct {
	settingService *SettingService
	verifier       TurnstileVerifier
}

// TurnstileVerifyResponse Cloudflare Turnstile 验证响应
type TurnstileVerifyResponse struct {
	Success          bool     `json:"success"`
	ChallengeTS      string   `json:"challenge_ts"`
	Hostname         string   `json:"hostname"`
	ErrorCodes       []string `json:"error-codes"`
	Action           string   `json:"action"`
	CData            string   `json:"cdata"`
	HTTPStatus       int      `json:"-"`
	VerifyDurationMS int64    `json:"-"`
}

// NewTurnstileService 创建 Turnstile 服务实例
func NewTurnstileService(settingService *SettingService, verifier TurnstileVerifier) *TurnstileService {
	return &TurnstileService{
		settingService: settingService,
		verifier:       verifier,
	}
}

// VerifyToken 验证 Turnstile token
func (s *TurnstileService) VerifyToken(ctx context.Context, token string, remoteIP string) error {
	// 检查是否启用 Turnstile
	if !s.settingService.IsTurnstileEnabled(ctx) {
		logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Disabled, skipping verification")
		return nil
	}

	// 获取 Secret Key
	secretKey := s.settingService.GetTurnstileSecretKey(ctx)
	if secretKey == "" {
		logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Secret key not configured")
		return ErrTurnstileNotConfigured
	}

	// 如果 token 为空，返回错误
	if token == "" {
		logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Token is empty")
		return ErrTurnstileVerificationFailed
	}

	sanitizedRemoteIP := ip.SanitizeTurnstileRemoteIP(remoteIP)
	remoteIPPrivate := remoteIP != "" && ip.IsPrivateOrLoopbackIP(remoteIP)
	logger.LegacyPrintf("service.turnstile", "[Turnstile] Verifying token, remoteip_passed=%v remote_ip_private=%v", sanitizedRemoteIP != "", remoteIPPrivate)
	started := time.Now()
	result, err := s.verifier.VerifyToken(ctx, secretKey, token, sanitizedRemoteIP)
	if err != nil {
		logger.With(zap.String("component", "service.turnstile")).Error("[Turnstile] Request failed",
			zap.Error(err),
			zap.Int64("duration_ms", time.Since(started).Milliseconds()),
			zap.Bool("remoteip_passed", sanitizedRemoteIP != ""),
			zap.String("remote_ip_redacted", redactRemoteIPForLog(remoteIP)),
			zap.Bool("remote_ip_private", remoteIPPrivate),
		)
		return fmt.Errorf("send request: %w", err)
	}

	if !result.Success {
		logger.With(zap.String("component", "service.turnstile")).Warn("[Turnstile] Verification failed",
			zap.Strings("error_codes", result.ErrorCodes),
			zap.String("hostname", result.Hostname),
			zap.String("challenge_ts", result.ChallengeTS),
			zap.String("action", result.Action),
			zap.Int("http_status", result.HTTPStatus),
			zap.Int64("duration_ms", result.VerifyDurationMS),
			zap.Bool("remoteip_passed", sanitizedRemoteIP != ""),
			zap.String("remote_ip_redacted", redactRemoteIPForLog(remoteIP)),
			zap.Bool("remote_ip_private", remoteIPPrivate),
		)
		return ErrTurnstileVerificationFailed
	}

	logger.LegacyPrintf("service.turnstile", "%s", "[Turnstile] Verification successful")
	return nil
}

func redactRemoteIPForLog(raw string) string {
	sanitized := ip.SanitizeTurnstileRemoteIP(raw)
	if sanitized == "" {
		if raw == "" {
			return ""
		}
		return "[redacted]"
	}
	parts := strings.Split(sanitized, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + "." + parts[2] + ".0"
	}
	if colon := strings.LastIndex(sanitized, ":"); colon > 0 {
		return sanitized[:colon] + ":"
	}
	return "[redacted]"
}

// IsEnabled 检查 Turnstile 是否启用
func (s *TurnstileService) IsEnabled(ctx context.Context) bool {
	return s.settingService.IsTurnstileEnabled(ctx)
}

// ValidateSecretKey 验证 Turnstile Secret Key 是否有效
func (s *TurnstileService) ValidateSecretKey(ctx context.Context, secretKey string) error {
	// 发送一个测试token的验证请求来检查secret_key是否有效
	result, err := s.verifier.VerifyToken(ctx, secretKey, "test-validation", "")
	if err != nil {
		return fmt.Errorf("validate secret key: %w", err)
	}

	// 检查是否有 invalid-input-secret 错误
	for _, code := range result.ErrorCodes {
		if code == "invalid-input-secret" {
			return ErrTurnstileInvalidSecretKey
		}
	}

	// 其他错误（如 invalid-input-response）说明 secret key 是有效的
	return nil
}
