package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// ImageGenerations handles OpenAI Images generation requests.
// POST /v1/images/generations
func (h *OpenAIGatewayHandler) ImageGenerations(c *gin.Context) {
	h.handleImages(c, "generations")
}

// ImageEdits handles OpenAI Images edit requests.
// POST /v1/images/edits
func (h *OpenAIGatewayHandler) ImageEdits(c *gin.Context) {
	h.handleImages(c, "edits")
}

func (h *OpenAIGatewayHandler) handleImages(c *gin.Context, operation string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	setOpenAIClientTransportHTTP(c)

	requestStart := time.Now()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.images",
		zap.String("operation", operation),
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	var body []byte
	reqModel := ""
	reqStream := false
	switch operation {
	case "generations":
		var err error
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
				return
			}
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
			return
		}
		if len(body) == 0 {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
			return
		}
		if !gjson.ValidBytes(body) {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
			return
		}
		modelResult := gjson.GetBytes(body, "model")
		if !modelResult.Exists() || modelResult.Type != gjson.String || strings.TrimSpace(modelResult.String()) == "" {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
			return
		}
		promptResult := gjson.GetBytes(body, "prompt")
		if !promptResult.Exists() || promptResult.Type != gjson.String || strings.TrimSpace(promptResult.String()) == "" {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "prompt is required")
			return
		}
		streamResult := gjson.GetBytes(body, "stream")
		if streamResult.Exists() && streamResult.Type != gjson.True && streamResult.Type != gjson.False {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "invalid stream field type")
			return
		}
		reqModel = strings.TrimSpace(modelResult.String())
		reqStream = streamResult.Bool()
	case "edits":
		if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse multipart form")
			return
		}
		reqModel = strings.TrimSpace(c.Request.FormValue("model"))
		if reqModel == "" {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
			return
		}
		if strings.TrimSpace(c.Request.FormValue("prompt")) == "" {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "prompt is required")
			return
		}
		reqStream = strings.EqualFold(strings.TrimSpace(c.Request.FormValue("stream")), "true")
	default:
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Unknown Images operation")
		return
	}
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	setOpsRequestContext(c, reqModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()
	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		reqLog.Info("openai_images.billing_eligibility_check_failed", zap.Error(err))
		status, code, message := billingErrorDetails(err)
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	deferredOAuthSelections := make([]*service.AccountSelectionResult, 0, 2)
	deferredOAuthSelectionIDs := make(map[int64]struct{})
	var lastEligibilityErr error
	var lastFailoverErr *service.UpstreamFailoverError

	for {
		var (
			selection        *service.AccountSelectionResult
			scheduleDecision service.OpenAIAccountScheduleDecision
			err              error
		)
		usedDeferredOAuth := false

		selection, scheduleDecision, err = h.gatewayService.SelectAccountWithSchedulerForFormat(
			c.Request.Context(),
			apiKey.GroupID,
			"",
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIAPIFormatResponses,
			service.OpenAIUpstreamTransportAny,
		)
		if err != nil && len(deferredOAuthSelections) > 0 {
			selection = deferredOAuthSelections[0]
			deferredOAuthSelections = deferredOAuthSelections[1:]
			usedDeferredOAuth = true
			err = nil
		}
		if err != nil {
			reqLog.Warn("openai_images.account_select_failed", zap.Error(err), zap.Int("excluded_account_count", len(failedAccountIDs)))
			if len(failedAccountIDs) == 0 {
				h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", streamStarted)
				return
			}
			if typedErr, ok := service.ResolveOpenAIOAuthImagesError(lastEligibilityErr); ok {
				h.handleStreamingAwareError(c, typedErr.Status, typedErr.ErrType, typedErr.Message, streamStarted)
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, streamStarted)
				return
			}
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available OpenAI accounts for Images API", streamStarted)
			return
		}
		if selection == nil || selection.Account == nil {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", streamStarted)
			return
		}
		account := selection.Account
		_ = scheduleDecision
		capability, eligibilityErr := h.gatewayService.ValidateOpenAIImagesAccount(c.Request.Context(), account, operation, reqStream)
		if eligibilityErr != nil {
			lastEligibilityErr = eligibilityErr
			failedAccountIDs[account.ID] = struct{}{}
			if typedErr, ok := service.ResolveOpenAIOAuthImagesError(eligibilityErr); ok {
				reqLog.Debug(
					"openai_images.skip_ineligible_account",
					zap.Int64("account_id", account.ID),
					zap.String("account_type", string(account.Type)),
					zap.String("reason_code", typedErr.Code),
					zap.String("reason_message", typedErr.Message),
				)
			} else {
				reqLog.Debug("openai_images.skip_ineligible_account", zap.Int64("account_id", account.ID), zap.String("account_type", string(account.Type)), zap.Error(eligibilityErr))
			}
			continue
		}
		if account.Type == service.AccountTypeOAuth && !usedDeferredOAuth {
			if _, seen := deferredOAuthSelectionIDs[account.ID]; !seen {
				deferredOAuthSelections = append(deferredOAuthSelections, selection)
				deferredOAuthSelectionIDs[account.ID] = struct{}{}
			}
			failedAccountIDs[account.ID] = struct{}{}
			reqLog.Debug(
				"openai_images.defer_oauth_account",
				zap.Int64("account_id", account.ID),
				zap.String("strategy", capability.Strategy),
				zap.String("reason", "prefer_apikey_first"),
			)
			continue
		}
		reqLog.Debug(
			"openai_images.account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.Bool("oauth_images_experimental", account.Type == service.AccountTypeOAuth),
		)
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, acquired := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, reqStream, &streamStarted, reqLog)
		if !acquired {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		writerSizeBeforeForward := c.Writer.Size()
		var result *service.OpenAIForwardResult
		var forwardErr error
		switch operation {
		case "generations":
			forwardBody := body
			if channelMapping.Mapped {
				forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
			}
			result, forwardErr = h.gatewayService.ForwardAsImageGeneration(c.Request.Context(), c, account, forwardBody, reqModel)
		case "edits":
			upstreamModel := reqModel
			if channelMapping.Mapped {
				upstreamModel = channelMapping.MappedModel
			}
			result, forwardErr = h.gatewayService.ForwardAsImageEdit(c.Request.Context(), c, account, reqModel, upstreamModel)
		}
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if forwardErr != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(forwardErr, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						select {
						case <-c.Request.Context().Done():
							return
						case <-time.After(sameAccountRetryDelay):
						}
						continue
					}
				}
				if c.Writer.Size() != writerSizeBeforeForward {
					h.handleFailoverExhausted(c, failoverErr, true)
					return
				}
				h.gatewayService.TempUnscheduleRetryableError(c.Request.Context(), account.ID, failoverErr)
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				switchCount++
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
			wroteFallback := h.ensureForwardErrorResponse(c, streamStarted)
			reqLog.Warn("openai_images.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Error(forwardErr),
			)
			return
		}
		if result != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
		}
		h.recordImagesUsage(c, reqLog, apiKey, subject.UserID, account, subscription, result, reqModel, channelMapping)
		reqLog.Debug("openai_images.request_completed", zap.Int64("account_id", account.ID), zap.Int("switch_count", switchCount))
		return
	}
}

func (h *OpenAIGatewayHandler) recordImagesUsage(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	userID int64,
	account *service.Account,
	subscription *service.UserSubscription,
	result *service.OpenAIForwardResult,
	reqModel string,
	channelMapping service.ChannelMappingResult,
) {
	if result == nil {
		return
	}
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	h.submitUsageRecordTask(func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    GetInboundEndpoint(c),
			UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			APIKeyService:      h.apiKeyService,
			ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.images"),
				zap.Int64("user_id", userID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Any("group_id", apiKey.GroupID),
				zap.String("model", reqModel),
				zap.Int64("account_id", account.ID),
			).Error("openai_images.record_usage_failed", zap.Error(err))
		}
	})
}
