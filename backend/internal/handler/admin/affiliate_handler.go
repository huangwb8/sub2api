package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AffiliateHandler struct {
	affiliateService *service.AffiliateService
	adminService     service.AdminService
}

func NewAffiliateHandler(affiliateService *service.AffiliateService, adminService service.AdminService) *AffiliateHandler {
	return &AffiliateHandler{affiliateService: affiliateService, adminService: adminService}
}

func (h *AffiliateHandler) ListUsers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	entries, total, err := h.affiliateService.AdminListCustomUsers(c.Request.Context(), service.AffiliateAdminFilter{
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, entries, total, page, pageSize)
}

type UpdateAffiliateUserRequest struct {
	AffCode              *string  `json:"aff_code"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent"`
	ClearRebateRate      bool     `json:"clear_rebate_rate"`
}

func (h *AffiliateHandler) UpdateUserSettings(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req UpdateAffiliateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.AffCode != nil {
		if err := h.affiliateService.AdminUpdateUserAffCode(c.Request.Context(), userID, *req.AffCode); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	if req.ClearRebateRate {
		if err := h.affiliateService.AdminSetUserRebateRate(c.Request.Context(), userID, nil); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	} else if req.AffRebateRatePercent != nil {
		if err := h.affiliateService.AdminSetUserRebateRate(c.Request.Context(), userID, req.AffRebateRatePercent); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	response.Success(c, gin.H{"user_id": userID})
}

func (h *AffiliateHandler) ClearUserSettings(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	if err := h.affiliateService.AdminSetUserRebateRate(c.Request.Context(), userID, nil); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if _, err := h.affiliateService.AdminResetUserAffCode(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"user_id": userID})
}

type BatchSetRateRequest struct {
	UserIDs              []int64  `json:"user_ids" binding:"required"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent"`
	Clear                bool     `json:"clear"`
}

func (h *AffiliateHandler) BatchSetRate(c *gin.Context) {
	var req BatchSetRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.UserIDs) == 0 {
		response.BadRequest(c, "user_ids cannot be empty")
		return
	}
	if !req.Clear && req.AffRebateRatePercent == nil {
		response.BadRequest(c, "aff_rebate_rate_percent is required unless clear=true")
		return
	}
	rate := req.AffRebateRatePercent
	if req.Clear {
		rate = nil
	}
	if err := h.affiliateService.AdminBatchSetUserRebateRate(c.Request.Context(), req.UserIDs, rate); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"affected": len(req.UserIDs)})
}

type AffiliateUserSummary struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (h *AffiliateHandler) LookupUsers(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		response.Success(c, []AffiliateUserSummary{})
		return
	}
	users, _, err := h.adminService.ListUsers(c.Request.Context(), 1, 20, service.UserListFilters{Search: keyword}, "email", "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]AffiliateUserSummary, len(users))
	for i, u := range users {
		result[i] = AffiliateUserSummary{ID: u.ID, Email: u.Email, Username: u.Username}
	}
	response.Success(c, result)
}

func (h *AffiliateHandler) GetUserOverview(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	overview, err := h.affiliateService.AdminGetUserOverview(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

func (h *AffiliateHandler) ListInviteRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseAffiliateRecordFilter(c, page, pageSize)
	items, total, err := h.affiliateService.AdminListInviteRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *AffiliateHandler) ListRebateRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseAffiliateRecordFilter(c, page, pageSize)
	items, total, err := h.affiliateService.AdminListRebateRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *AffiliateHandler) ListTransferRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseAffiliateRecordFilter(c, page, pageSize)
	items, total, err := h.affiliateService.AdminListTransferRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func parseAffiliateRecordFilter(c *gin.Context, page, pageSize int) service.AffiliateRecordFilter {
	filter := service.AffiliateRecordFilter{
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
		SortBy:   c.Query("sort_by"),
		SortDesc: !strings.EqualFold(c.Query("sort_order"), "asc"),
	}
	userTZ := c.Query("timezone")
	if t := parseAffiliateRecordStartTime(c.Query("start_at"), userTZ); t != nil {
		filter.StartAt = t
	}
	if t := parseAffiliateRecordEndTime(c.Query("end_at"), userTZ); t != nil {
		filter.EndAt = t
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return filter
}

func parseAffiliateRecordStartTime(raw string, userTZ string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ); err == nil {
		return &parsed
	}
	return nil
}

func parseAffiliateRecordEndTime(raw string, userTZ string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return &parsed
	}
	if parsed, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ); err == nil {
		end := parsed.AddDate(0, 0, 1).Add(-time.Nanosecond)
		return &end
	}
	return nil
}
