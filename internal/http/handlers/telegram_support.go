package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/support"
	"github.com/gin-gonic/gin"
)

type telegramSupportCreateRequest struct {
	TelegramID int64  `json:"telegram_id" binding:"required"`
	Subject    string `json:"subject" binding:"required"`
	Message    string `json:"message" binding:"required"`
}

type telegramSupportListRequest struct {
	TelegramID int64  `json:"telegram_id" binding:"required"`
	Status     string `json:"status"`
}

type telegramSupportDetailRequest struct {
	TelegramID int64  `json:"telegram_id" binding:"required"`
	TicketID   string `json:"ticket_id" binding:"required"`
}

type telegramSupportReplyRequest struct {
	TelegramID int64  `json:"telegram_id" binding:"required"`
	TicketID   string `json:"ticket_id" binding:"required"`
	Message    string `json:"message" binding:"required"`
}

func (h TelegramHandler) SupportCreate(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramSupportCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.supportService.CreateFromTelegram(c.Request.Context(), support.CreateFromTelegramInput{
		TelegramID: req.TelegramID,
		Subject:    req.Subject,
		Message:    req.Message,
	})
	if err != nil {
		h.writeSupportError(c, err, "failed to create telegram support ticket")
		return
	}

	response.Success(c, http.StatusOK, "telegram support ticket created successfully", result)
}

func (h TelegramHandler) SupportList(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramSupportListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.supportService.ListForTelegram(c.Request.Context(), support.ListForTelegramInput{
		TelegramID: req.TelegramID,
		Status:     req.Status,
	})
	if err != nil {
		h.writeSupportError(c, err, "failed to fetch telegram support tickets")
		return
	}

	response.Success(c, http.StatusOK, "telegram support tickets fetched successfully", gin.H{
		"items": result,
	})
}

func (h TelegramHandler) SupportDetail(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramSupportDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.supportService.GetForTelegram(c.Request.Context(), support.GetForTelegramInput{
		TelegramID: req.TelegramID,
		TicketID:   req.TicketID,
	})
	if err != nil {
		h.writeSupportError(c, err, "failed to fetch telegram support ticket detail")
		return
	}

	response.Success(c, http.StatusOK, "telegram support ticket detail fetched successfully", result)
}

func (h TelegramHandler) SupportReply(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramSupportReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.supportService.ReplyFromTelegram(c.Request.Context(), support.ReplyFromTelegramInput{
		TelegramID: req.TelegramID,
		TicketID:   req.TicketID,
		Message:    req.Message,
	})
	if err != nil {
		h.writeSupportError(c, err, "failed to reply telegram support ticket")
		return
	}

	response.Success(c, http.StatusOK, "telegram support ticket replied successfully", result)
}

func (h TelegramHandler) writeSupportError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, support.ErrInvalidTicketRequest):
		response.Fail(c, http.StatusBadRequest, "invalid support ticket request", "bad_request", nil)
	case errors.Is(err, support.ErrTelegramUserNotFound):
		response.Fail(c, http.StatusNotFound, "telegram user not found", "not_found", nil)
	case errors.Is(err, support.ErrTicketNotFound):
		response.Fail(c, http.StatusNotFound, "support ticket not found", "not_found", nil)
	case errors.Is(err, support.ErrTicketClosed):
		response.Fail(c, http.StatusBadRequest, "support ticket is closed", "bad_request", nil)
	case errors.Is(err, support.ErrInvalidStatus):
		response.Fail(c, http.StatusBadRequest, "invalid support ticket status", "bad_request", nil)
	default:
		response.Fail(c, http.StatusInternalServerError, fallback, "internal_server_error", nil)
	}
}
