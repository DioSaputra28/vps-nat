package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/adminops"
	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/gin-gonic/gin"
)

type AdminOpsHandler struct {
	service *adminops.Service
}

type createServerCostRequest struct {
	NodeID       string  `json:"node_id" binding:"required"`
	PurchaseCost int64   `json:"purchase_cost" binding:"required"`
	Notes        *string `json:"notes"`
	RecordedAt   *string `json:"recorded_at"`
}

type walletAdjustmentRequest struct {
	UserID         string `json:"user_id" binding:"required"`
	AdjustmentType string `json:"adjustment_type" binding:"required"`
	Amount         int64  `json:"amount" binding:"required"`
	Reason         string `json:"reason" binding:"required"`
}

func NewAdminOpsHandler(service *adminops.Service) AdminOpsHandler {
	return AdminOpsHandler{service: service}
}

func (h AdminOpsHandler) DashboardOverview(c *gin.Context) {
	result, err := h.service.DashboardOverview(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to fetch dashboard overview", "internal_server_error", nil)
		return
	}
	response.Success(c, http.StatusOK, "dashboard overview fetched successfully", result)
}

func (h AdminOpsHandler) ActivityLogList(c *gin.Context) {
	page, err := parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "page must be a positive integer", "bad_request", nil)
		return
	}
	limit, err := parsePositiveInt(c.Query("limit"), 10)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "limit must be a positive integer", "bad_request", nil)
		return
	}

	result, err := h.service.ListActivityLogs(c.Request.Context(), adminops.ListActivityLogsInput{
		Page:       page,
		Limit:      limit,
		ActorType:  c.Query("actor_type"),
		Action:     c.Query("action"),
		TargetType: c.Query("target_type"),
		TargetID:   c.Query("target_id"),
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to fetch activity logs", "internal_server_error", nil)
		return
	}
	response.Success(c, http.StatusOK, "activity logs fetched successfully", result)
}

func (h AdminOpsHandler) FinanceSummary(c *gin.Context) {
	result, err := h.service.FinanceSummary(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to fetch finance summary", "internal_server_error", nil)
		return
	}
	response.Success(c, http.StatusOK, "finance summary fetched successfully", result)
}

func (h AdminOpsHandler) ServerCostList(c *gin.Context) {
	result, err := h.service.ListServerCosts(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to fetch server costs", "internal_server_error", nil)
		return
	}
	response.Success(c, http.StatusOK, "server costs fetched successfully", result)
}

func (h AdminOpsHandler) ServerCostCreate(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	if !middleware.HasRole(admin, "super_admin") {
		response.Fail(c, http.StatusForbidden, "forbidden", "forbidden", nil)
		return
	}

	var req createServerCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	var recordedAt *time.Time
	if req.RecordedAt != nil && *req.RecordedAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.RecordedAt)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "recorded_at must be RFC3339 format", "bad_request", nil)
			return
		}
		recordedAt = &parsed
	}

	result, err := h.service.CreateServerCost(c.Request.Context(), admin, adminops.CreateServerCostInput{
		NodeID:       req.NodeID,
		PurchaseCost: req.PurchaseCost,
		Notes:        req.Notes,
		RecordedAt:   recordedAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminops.ErrInvalidServerCost), errors.Is(err, adminops.ErrInvalidRequest):
			response.Fail(c, http.StatusBadRequest, "invalid server cost input", "bad_request", nil)
		case errors.Is(err, adminops.ErrNotFound):
			response.Fail(c, http.StatusNotFound, "node not found", "not_found", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to create server cost", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "server cost created successfully", gin.H{
		"server_cost": result,
	})
}

func (h AdminOpsHandler) WalletAdjustment(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	if !middleware.HasRole(admin, "admin", "super_admin") {
		response.Fail(c, http.StatusForbidden, "forbidden", "forbidden", nil)
		return
	}

	var req walletAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.AdjustWallet(c.Request.Context(), admin, adminops.WalletAdjustmentInput{
		UserID:         req.UserID,
		AdjustmentType: req.AdjustmentType,
		Amount:         req.Amount,
		Reason:         req.Reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, adminops.ErrInvalidRequest), errors.Is(err, adminops.ErrInvalidAdjustment):
			response.Fail(c, http.StatusBadRequest, "invalid wallet adjustment input", "bad_request", nil)
		case errors.Is(err, adminops.ErrNotFound):
			response.Fail(c, http.StatusNotFound, "user not found", "not_found", nil)
		case errors.Is(err, adminops.ErrInsufficientBalance):
			response.Fail(c, http.StatusConflict, "insufficient wallet balance", "conflict", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to adjust wallet", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "wallet adjusted successfully", result)
}

func (h AdminOpsHandler) AlertList(c *gin.Context) {
	page, err := parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "page must be a positive integer", "bad_request", nil)
		return
	}
	limit, err := parsePositiveInt(c.Query("limit"), 10)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "limit must be a positive integer", "bad_request", nil)
		return
	}

	result, err := h.service.ListAlerts(c.Request.Context(), adminops.ListAlertsInput{
		Page:      page,
		Limit:     limit,
		Status:    c.Query("status"),
		AlertType: c.Query("alert_type"),
		ServiceID: c.Query("service_id"),
		NodeID:    c.Query("node_id"),
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to fetch monitoring alerts", "internal_server_error", nil)
		return
	}
	response.Success(c, http.StatusOK, "monitoring alerts fetched successfully", result)
}
