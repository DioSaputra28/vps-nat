package handlers

import (
	"errors"
	"net/http"

	"github.com/DioSaputra28/vps-nat/internal/http/response"
	"github.com/DioSaputra28/vps-nat/internal/telegram"
	"github.com/gin-gonic/gin"
)

type telegramBuyVPSSubmitRequest struct {
	TelegramID    int64  `json:"telegram_id" binding:"required"`
	PackageID     string `json:"package_id" binding:"required"`
	ImageAlias    string `json:"image_alias" binding:"required"`
	Hostname      string `json:"hostname" binding:"required"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

type telegramBuyVPSStatusRequest struct {
	TelegramID int64  `json:"telegram_id" binding:"required"`
	OrderID    string `json:"order_id" binding:"required"`
}

type telegramWalletTopupSubmitRequest struct {
	TelegramID int64 `json:"telegram_id" binding:"required"`
	Amount     int64 `json:"amount" binding:"required"`
}

type telegramWalletTopupStatusRequest struct {
	TelegramID int64  `json:"telegram_id" binding:"required"`
	TopupID    string `json:"topup_id" binding:"required"`
}

func (h TelegramHandler) BuyVPSSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramBuyVPSSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.BuyVPSSubmit(c.Request.Context(), telegram.BuyVPSSubmitInput{
		TelegramID:    req.TelegramID,
		PackageID:     req.PackageID,
		ImageAlias:    req.ImageAlias,
		Hostname:      req.Hostname,
		PaymentMethod: req.PaymentMethod,
	})
	if err != nil {
		switch {
		case errors.Is(err, telegram.ErrInvalidTelegramUser), errors.Is(err, telegram.ErrInvalidActionRequest), errors.Is(err, telegram.ErrUnsupportedPayment):
			response.Fail(c, http.StatusBadRequest, "invalid buy vps request", "bad_request", nil)
		case errors.Is(err, telegram.ErrTelegramUserNotFound), errors.Is(err, telegram.ErrPackageNotFound):
			response.Fail(c, http.StatusNotFound, "resource not found", "not_found", nil)
		case errors.Is(err, telegram.ErrHostnameAlreadyExists), errors.Is(err, telegram.ErrInsufficientBalance):
			response.Fail(c, http.StatusConflict, err.Error(), "conflict", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to submit buy vps request", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "telegram buy vps submitted successfully", result)
}

func (h TelegramHandler) BuyVPSStatus(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramBuyVPSStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.BuyVPSStatus(c.Request.Context(), telegram.BuyVPSStatusInput{
		TelegramID: req.TelegramID,
		OrderID:    req.OrderID,
	})
	if err != nil {
		switch {
		case errors.Is(err, telegram.ErrInvalidActionRequest), errors.Is(err, telegram.ErrInvalidTelegramUser):
			response.Fail(c, http.StatusBadRequest, "invalid buy vps status request", "bad_request", nil)
		case errors.Is(err, telegram.ErrOrderNotFound):
			response.Fail(c, http.StatusNotFound, "order not found", "not_found", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to fetch buy vps status", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "telegram buy vps status fetched successfully", result)
}

func (h TelegramHandler) WalletTopupSubmit(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramWalletTopupSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.WalletTopupSubmit(c.Request.Context(), telegram.WalletTopupSubmitInput{
		TelegramID: req.TelegramID,
		Amount:     req.Amount,
	})
	if err != nil {
		switch {
		case errors.Is(err, telegram.ErrInvalidTelegramUser), errors.Is(err, telegram.ErrInvalidActionRequest), errors.Is(err, telegram.ErrUnsupportedPayment):
			response.Fail(c, http.StatusBadRequest, "invalid wallet topup request", "bad_request", nil)
		case errors.Is(err, telegram.ErrTelegramUserNotFound):
			response.Fail(c, http.StatusNotFound, "telegram user not found", "not_found", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to submit wallet topup request", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "telegram wallet topup submitted successfully", result)
}

func (h TelegramHandler) WalletTopupStatus(c *gin.Context) {
	if !h.authorize(c) {
		return
	}

	var req telegramWalletTopupStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request body", "bad_request", nil)
		return
	}

	result, err := h.service.WalletTopupStatus(c.Request.Context(), telegram.WalletTopupStatusInput{
		TelegramID: req.TelegramID,
		TopupID:    req.TopupID,
	})
	if err != nil {
		switch {
		case errors.Is(err, telegram.ErrInvalidActionRequest), errors.Is(err, telegram.ErrInvalidTelegramUser):
			response.Fail(c, http.StatusBadRequest, "invalid wallet topup status request", "bad_request", nil)
		case errors.Is(err, telegram.ErrWalletTopupNotFound):
			response.Fail(c, http.StatusNotFound, "wallet topup not found", "not_found", nil)
		default:
			response.Fail(c, http.StatusInternalServerError, "failed to fetch wallet topup status", "internal_server_error", nil)
		}
		return
	}

	response.Success(c, http.StatusOK, "telegram wallet topup status fetched successfully", result)
}
