package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

// JWTAuthMiddleware JWT 认证中间件类型
type JWTAuthMiddleware gin.HandlerFunc

// AdminAuthMiddleware 管理员认证中间件类型
type AdminAuthMiddleware gin.HandlerFunc

// APIKeyAuthMiddleware API Key 认证中间件类型
type APIKeyAuthMiddleware gin.HandlerFunc

func ProvideJWTAuthMiddleware(
	authService *service.AuthService,
	userService *service.UserService,
	riskService *service.UserRiskService,
) JWTAuthMiddleware {
	return NewJWTAuthMiddleware(authService, userService, riskService)
}

// ProvideAPIKeyAuthMiddleware gives Wire a concrete GatewayRPMCache dependency
// instead of the variadic form used by tests and lightweight call sites.
func ProvideAPIKeyAuthMiddleware(
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	cfg *config.Config,
	riskService *service.UserRiskService,
	signalService *service.UserRiskSignalService,
	rpmCache service.GatewayRPMCache,
) APIKeyAuthMiddleware {
	return NewAPIKeyAuthMiddleware(apiKeyService, subscriptionService, cfg, riskService, signalService, rpmCache)
}

// ProviderSet 中间件层的依赖注入
var ProviderSet = wire.NewSet(
	ProvideJWTAuthMiddleware,
	NewAdminAuthMiddleware,
	ProvideAPIKeyAuthMiddleware,
)
