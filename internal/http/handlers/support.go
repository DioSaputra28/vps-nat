package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/http/middleware"
	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/support"
	"github.com/gin-gonic/gin"
)

type SupportHandler struct {
	service *support.Service
}

type adminSupportReplyRequest struct {
	Message string `json:"message" binding:"required"`
}

type adminSupportStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func NewSupportHandler(service *support.Service) SupportHandler {
	return SupportHandler{service: service}
}

func (h SupportHandler) List(c *gin.Context) {
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

	result, err := h.service.ListForAdmin(c.Request.Context(), support.ListForAdminInput{
		Page:   page,
		Limit:  limit,
		Status: c.Query("status"),
		Search: c.Query("search"),
	})
	if err != nil {
		h.writeSupportAdminError(c, err, "failed to list support tickets")
		return
	}

	response.Success(c, http.StatusOK, "support tickets fetched successfully", gin.H{
		"items": result.Items,
		"meta": gin.H{
			"page":        result.Page,
			"limit":       result.Limit,
			"total_items": result.TotalItems,
			"total_pages": result.TotalPages,
			"status":      result.Status,
			"search":      result.Search,
		},
	})
}

func (h SupportHandler) GetByID(c *gin.Context) {
	result, err := h.service.GetForAdmin(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeSupportAdminError(c, err, "failed to get support ticket")
		return
	}

	response.Success(c, http.StatusOK, "support ticket fetched successfully", result)
}

func (h SupportHandler) Reply(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	var req adminSupportReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.ReplyFromAdmin(c.Request.Context(), support.ReplyFromAdminInput{
		TicketID: c.Param("id"),
		Admin:    admin,
		Message:  req.Message,
	})
	if err != nil {
		h.writeSupportAdminError(c, err, "failed to reply support ticket")
		return
	}

	response.Success(c, http.StatusOK, "support ticket replied successfully", result)
}

func (h SupportHandler) UpdateStatus(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	var req adminSupportStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.UpdateStatus(c.Request.Context(), support.UpdateStatusInput{
		TicketID: c.Param("id"),
		Admin:    admin,
		Status:   req.Status,
	})
	if err != nil {
		h.writeSupportAdminError(c, err, "failed to update support ticket status")
		return
	}

	response.Success(c, http.StatusOK, "support ticket status updated successfully", result)
}

func (h SupportHandler) writeSupportAdminError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, support.ErrInvalidTicketRequest), errors.Is(err, support.ErrInvalidStatus):
		response.Fail(c, http.StatusBadRequest, "invalid support ticket request", "bad_request", nil)
	case errors.Is(err, support.ErrTicketNotFound):
		response.Fail(c, http.StatusNotFound, "support ticket not found", "not_found", nil)
	case errors.Is(err, support.ErrTicketClosed):
		response.Fail(c, http.StatusBadRequest, "support ticket is closed", "bad_request", nil)
	default:
		response.Fail(c, http.StatusInternalServerError, fallback, "internal_server_error", nil)
	}
}
